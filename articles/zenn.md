---
title: "Go Wasm フルスタック向けの型安全 HTTP ライブラリを作った"
emoji: "🔗"
type: "tech"
topics: ["go", "wasm", "webassembly", "http", "generics"]
published: false
---

## 「API の型が合わない」を、実行前に潰したい

Go で WebAssembly（Wasm）フロントエンドを書いていると、バックエンドと API の型定義を共有できるという強みがあります。でも「共有できる」だけでは不十分で、**呼び出し側でスキーマの不一致をコンパイル時に弾けるか**が実際の品質に直結します。

既存の選択肢を調べてみると：

- **gRPC / connect-go** — protobuf とコード生成が必要
- **huma** — サーバー専用。クライアント側の型安全は別途対応が必要
- **素の `net/http` + 手書き** — 型は共有できるが、フィールド名のミスは実行時まで気づかない

「コード生成なし・外部依存なし・フロントとバックで型を共有して、コンパイル時に不一致を検出できる」ものが欲しかったので、自分で作りました。それが **apibind** です。

https://github.com/yuma-seno/apibind

---

## 使い方

### 1. 共有パッケージで API を定義する

```go
// shared/api.go
// フロントエンドとバックエンドの両方がこのパッケージをインポートする
package shared

import apibind "github.com/yuma-seno/apibind"

type UpdateUserRequest struct {
    ID   string
    Name string `json:"name"`
}

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

var UpdateUserAPI = apibind.Endpoint[UpdateUserRequest, User]{
    Method: apibind.MethodPut,
    Path: apibind.NewPath[UpdateUserRequest]().
        S("/api/users/").
        P("id", func(r *UpdateUserRequest) *string { return &r.ID }),
}
```

`P()` に渡す `func(*Req) *string` がポイントです。**フィールドへのポインタを返す関数**なので、存在しないフィールドを指定するとコンパイルエラーになります。フィールド名の typo は実行前に検出できます。

### 2. バックエンドでハンドラを登録する

```go
// cmd/backend/main.go
mux.HandleFunc(shared.UpdateUserAPI.RoutePattern(), shared.UpdateUserAPI.Handler(
    func(r *http.Request, req shared.UpdateUserRequest) (shared.User, error) {
        // req.ID   → URL パスから自動で取得済み
        // req.Name → JSON ボディから自動でデコード済み
        return db.UpdateUser(req.ID, req.Name)
    },
))
```

`RoutePattern()` は `"PUT /api/users/{id}"` を返します。Go 1.22 の `http.ServeMux` にそのまま渡せます。

### 3. フロントエンド（Go Wasm）から呼び出す

```go
// cmd/frontend/main.go
//go:build js && wasm

var client = apibind.NewClient("") // 空文字 = same origin

resp, err := apibind.Call(ctx, client, shared.UpdateUserAPI, shared.UpdateUserRequest{
    ID:   "123",
    Name: "Alice",
})
// → PUT /api/users/123
//   Body: {"name":"Alice"}  （ID はパスに使われるので body から自動で除外）
```

`Call()` は `context.Context` を受け取るので、タイムアウトやキャンセルが標準的な Go のやり方でそのまま使えます。

---

## パラメータのルーティング

`apibind` はメソッドとパス定義に応じて、リクエストのフィールドを自動的に正しい場所に振り分けます。

| | `GET` / `DELETE` | `POST` / `PUT` / `PATCH` |
|---|---|---|
| `P()` で定義したフィールド | URL パスに埋め込み | URL パスに埋め込み |
| それ以外のフィールド | クエリパラメータ | JSON ボディ |

### クエリパラメータの例（GET）

ポインタ型（`*int` など）にすると **nil のとき URL から省略**されます。省略可能なフィルタ条件を表現するのに便利です。

```go
type ListUsersRequest struct {
    Page  *int    `json:"page"`   // nil なら ?page= は付かない
    Limit *int    `json:"limit"`
    Name  *string `json:"name"`
}

var ListUsersAPI = apibind.Endpoint[ListUsersRequest, []User]{
    Method: apibind.MethodGet,
    Path:   apibind.NewPath[ListUsersRequest]().S("/api/users"),
}
```

```go
// フロントエンド
page := 1
resp, err := apibind.Call(ctx, client, shared.ListUsersAPI, shared.ListUsersRequest{
    Page: &page,
    // Limit, Name は nil なので URL に含まれない
})
// → GET /api/users?page=1
```

バックエンドでは `SetQueryParams` が `r.URL.Query().Get` を使って自動でフィールドにセットします。

---

## エラーハンドリング

```go
var apiErr *apibind.APIError
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.StatusCode, apiErr.Message)
}

// よく使うステータスコード向けのセンチネルエラー
errors.Is(err, apibind.ErrBadRequest)   // 400
errors.Is(err, apibind.ErrNotFound)     // 404
errors.Is(err, apibind.ErrServerError)  // 500
```

ハンドラから HTTP エラーを返すには：

```go
return User{}, &apibind.APIError{StatusCode: http.StatusNotFound, Message: "user not found"}
```

---

## 類似ライブラリとの比較

| | apibind | connect-go | huma |
|---|---|---|---|
| 型安全 | ✅ | ✅ | ✅ |
| コード生成 | ❌ 不要 | protobuf 必要 | ❌ 不要 |
| Go Wasm クライアント | ✅ | 🔺 使えるが重い | ❌ サーバー専用 |
| 外部依存 | なし | あり | あり |
| クライアント/サーバーで型共有 | ✅ | ✅（proto） | ❌ |
| context.Context | ✅ | ✅ | ✅ |

TypeScript の [tRPC](https://trpc.io/) に近い思想です。プロトコル定義ファイルやコード生成なしに、型安全な RPC を実現します。

---

## 設計について

`apibind` を作るにあたって意識したのは **「小さく保つ」** こと。コアの実装は 400 行程度で、ファイルは 5 つだけです。

- **`Endpoint[Req, Resp]`** — API 契約。共有パッケージに置く
- **`Call(ctx, client, ep, req)`** — クライアント側。フロントエンドのみが使う
- **`ep.Handler(fn)`** — サーバー側。`http.HandlerFunc` を返すのでどの `net/http` 互換フレームワークでも使える

パスパラメータの定義に使う `func(*Req) *string` は最初「冗長すぎる？」と思っていましたが、これが型安全性の要です。文字列ではなくポインタを返す関数にすることで、コンパイラが存在しないフィールドへの参照を弾いてくれます。

---

## インストール

Go 1.23.0 以上が必要です。

```sh
go get github.com/yuma-seno/apibind@v0.2.0
```

コントリビューションや Issue もお待ちしています。  
→ [CONTRIBUTING.md](https://github.com/yuma-seno/apibind/blob/main/CONTRIBUTING.md)

https://github.com/yuma-seno/apibind
