# 仮登録 /auth/register/initial （１）

# 目標
- ユーザーの仮登録を実装する

今回から仮登録処理を行なっていきます。

## 全体の流れの確認

- 仮登録 /auth/register/initial
    - クライアントからemail, passwordを受け取る
    - email宛に本人確認トークンを送信する
- 本登録 /auth/register/complete
    - クライアントからemailと本人確認トークンを受け取る
    - ユーザーの本登録を行う
- ログイン /auth/login
    - クライアントからemail, passwordを受け取る
    - 認証トークンとしてJWTを返す
- ユーザー情報の取得 /restricted/user/me
    - クライアントからJWTを受け取る
    - ユーザー情報を返す

こんな感じでした。

では、仮登録にはどんな機能が必要になるでしょうか。

## 必要な機能を考えよう！

箇条書きしてみます。

- クライアントから受け取ったemail, passwordのバリデーションが必要。
    - バリデーションっていうのは、ちゃんとしたメールアドレスのフォーマットになっているか、パスワードの長さはちゃんとしてるか、などチェックすることです。
    - バリデーションに関しては自分で実装するのは危ないのでgo-playground/validatorを使います。
- すでにユーザーがアクティブな場合、エラーを返す必要がある
- 仮登録状態のユーザーのemailを受け取った場合はどうする？
    - ユーザーは一度削除して、仮登録をやり直す
- メール送信機能が必要だ
- 本人確認用のトークンを作成する
    - トークンの長さは？
        - ８文字
    - トークンの有効期限はどうやって検証しようか？
        - ユーザーの作成日時を元に30分
- レスポンスを作成する

ただ、このまま１つのファイルに書き出すとどこに何があるかわからなくなるので役割でまとめてみます。

| パッケージ | 役割 | 機能 |
|:-----------|:------------|:------------|
| Repository       | DBとのやりとり        | ・emailからユーザーを取得する<br/>・ユーザーを仮登録で保存する<br/>・ユーザーを削除する         |
| Usecase     | 仮登録処理を行う	      | ・ユーザーがアクティブかどうか確認する<br/>・アクティブ（本登録）ならエラー<br/>・非アクティブ（仮登録）なら削除して仮登録処理をやり直す<br/>・本人確認トークンの作成<br/>・メール送信       |
| Handler       | リクエストボディの取得<br/>レスポンスの作成        | ・リクエストボディの取得<br/>・リクエストボディの検証<br/>・emailのフォーマット検証<br/>・パスワードの長さ検証（６〜２０文字）<br/>・レスポンスの作成         |

### Userを作成する
まずはUserを作成する必要があります。
entityディレクトリを作成し、次のファイルを作成してください。

entity/user.go
```
package entity

import "time"

type User struct {
	ID            UserID    `db:"id"`
	Email         string    `db:"email"`
	Salt          string    `db:"salt"`
	State         UserState `db:"state"`
	Password      Password  `db:"password"`
	ActivateToken string    `db:"activate_token"`
	UpdatedAt     time.Time `db:"updated_at"`
	CreatedAt     time.Time `db:"created_at"`
}

type Users []*User

type UserID uint64

type Password string

func (p Password) String() string {
	return "xxxxxxxxx"
}

func (p Password) GoString() string {
	return "xxxxxxxxx"
}

type UserState string

const (
	UserActive   = UserState("active")
	UserInactive = UserState("inactive")
)

func (u User) IsActive() bool {
	return u.State == UserActive
}

// パスワード＋ソルトをハッシュ化する
func (u *User) CreateHashedPassword(pw, salt string) (Password, error) {
	var b bytes.Buffer
	b.Write([]byte(pw))
	b.Write([]byte(salt))
	hashed, err := bcrypt.GenerateFromPassword(b.Bytes(), bcrypt.DefaultCost)
	return Password(hashed), err
}

// パスワードが正しいか検証する。
func (u User) Authenticate(pw string) error {
	var b bytes.Buffer
	b.Write([]byte(pw))
	b.Write([]byte(u.Salt))
	return bcrypt.CompareHashAndPassword([]byte(u.Password), b.Bytes())
}
```

### entity/user.go の解説
```
type User struct {
	ID            UserID    `db:"id"`
	Email         string    `db:"email"`
	Salt          string    `db:"salt"`
	State         UserState `db:"state"`
	Password      Password  `db:"password"`
	ActivateToken string    `db:"activate_token"`
	UpdatedAt     time.Time `db:"updated_at"`
	CreatedAt     time.Time `db:"created_at"`
}
```
- db:"~"はMySQLのuserテーブルの各カラムに対応している。sqlxを使う予定なので必須。
```
type UserID uint64
```
- Get(id uint64)のような関数があった時、これだけだとなんのidかわからない時がある
- Get(id UserID)とすることで、「あ、UserのIDが必要なんだな」とわかる
```
type Password string

func (p Password) String() string {
	return "xxxxxxxxx"
}

func (p Password) GoString() string {
	return "xxxxxxxxx"
}
```
- log.Printf("user=%v", user) や log.Printf("user=%#v", user) でユーザーのパスワードがそのままログに出力されてしまう
- そうならないためのマスキング

```
type UserState string

const (
	UserActive   = UserState("active")
	UserInactive = UserState("inactive")
)

func (u User) IsActive() bool {
	return u.State == UserActive
}
```

- Stateをstringで管理するとタイポした時が怖いので（例えば u.State = "actvie" みたいに）

```
// パスワード＋ソルトをハッシュ化する
func (u *User) CreateHashedPassword(pw, salt string) (Password, error) {
	var b bytes.Buffer
	b.Write([]byte(pw))
	b.Write([]byte(salt))
	hashed, err := bcrypt.GenerateFromPassword(b.Bytes(), bcrypt.DefaultCost)
	return Password(hashed), err
}

// パスワードが正しいか検証する。
func (u User) Authenticate(pw string) error {
	var b bytes.Buffer
	b.Write([]byte(pw))
	b.Write([]byte(u.Salt))
	return bcrypt.CompareHashAndPassword([]byte(u.Password), b.Bytes())
}
```

- ハッシュ化とは？
    - データをランダムな文字列に変換すること
    - ハッシュ化した文字列から元のデータはわからない
    - 元のデータとハッシュ化した文字列を比較すると元のデータとハッシュ化した文字列の元々のデータが同じだったかどうかはわかる
- なんでパスワードをハッシュ化するの？
    - パスワードをそのまま保存すると管理者が悪用できてしまう\
    - ハッシュ化すれば元のパスワードがわからないので悪用を防げる
- ソルトってなに？
    - 「ハッシュ化すれば元のパスワードがわからないからもう安心！」とはならない
    - よくあるパスワード（例えばpass1234とか）だとハッシュ化した文字列から元のパスワードがわかってしまう場合がある
    - なのでパスワードにさらにランダムな文字列を付け加えて、それをハッシュ化することで安全性を高めている
    - pass1234のハッシュ化した文字列はバレるかもしれないがpass1234にDdI5uz0Ruo0を付け加えpass1234DdI5uz0Ruo0をハッシュ化したものならバレないだろう、という考え
    - この時付け加えるランダムな文字列DdI5uz0Ruo0がソルト。ユーザーごとに異なるようにする。
- ハッシュ化の例
    - pass1234→bd94dcda26fccb4e68d6a31f9b5aac0b571ae266d822620e901ef7ebe3a11d4f
    - pass1234DdI5uz0Ruo0→d5856a9bbd24e0607e16dcbf117e6030ccd2911916cc7156f4e9610c2a6278f4

## Repository

repositoryディレクトリを作成しましょう！
repositorに必要なのは次の機能でした。

- emailからユーザーを取得する
- ユーザーを仮登録で保存する
- ユーザーを削除する

実装していきましょう！

repository/user_repository.go
```
package repository

import (
	"context"
	"fmt"
	"login-example/entity"
	"time"

	"github.com/jmoiron/sqlx"
)

type IUserRepository interface {
	PreRegister(ctx context.Context, u *entity.User) error
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	Delete(ctx context.Context, id entity.UserID) error
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) IUserRepository {
	return &userRepository{db: db}
}

// ユーザーをstate=inactiveで保存する
func (r *userRepository) PreRegister(ctx context.Context, u *entity.User) error {
	u.UpdatedAt = time.Now()
	u.CreatedAt = time.Now()
	u.State = entity.UserInactive

	query := `INSERT INTO user (
		email, password, salt, activate_token, state, updated_at, created_at
	) VALUES (:email, :password, :salt, :activate_token, :state, :updated_at, :created_at)`
	result, err := r.db.NamedExecContext(ctx, query, u)
	if err != nil {
		return fmt.Errorf("failed to Exec: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to LastInsertId: %w", err)
	}

	u.ID = entity.UserID(id)
	return nil
}

// emailからユーザーを取得する、対象のユーザーが存在しなかった場合、user=nilではないので注意
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	query := `SELECT 
		id, email, password, salt, state, activate_token, updated_at, created_at
		FROM user WHERE email = ?`
	u := &entity.User{}
	// 対象のユーザーが存在しない場合、sql.ErrNoRowsがエラーで返ってくる
	if err := r.db.GetContext(ctx, u, query, email); err != nil {
		return nil, fmt.Errorf("failed to get: %w", err)
	}
	return u, nil
}

// ユーザーを削除する
func (r *userRepository) Delete(ctx context.Context, id entity.UserID) error {
	query := `DELETE FROM user WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}
```

## Mail

Usecaseの前にメール送信をするためのMailerを作成します。

mail/mailer.go
```
package mail

import (
	"fmt"
	"net/smtp"
	"strings"
)

type IMailer interface {
	SendWithActivateToken(email, token string) error
}

func NewMailhogMailer() IMailer {
	return &mailhogMailer{}
}

type mailhogMailer struct {
}

// mailhog
var (
	hostname = "mail"
	port     = 1025
	username = "user@example.com"
	password = "password"
)

func (m *mailhogMailer) SendWithActivateToken(email, token string) error {
	from := "info@login-example.app"
	recipients := []string{email}
	subject := "認証コード by login-example"
	body := fmt.Sprintf("認証用トークンです。\nトークン: %s", token)

	smtpServer := fmt.Sprintf("%s:%d", hostname, port)

	auth := smtp.CRAMMD5Auth(username, password)

	msg := []byte(strings.ReplaceAll(fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\n\n%s", from, strings.Join(recipients, ","), subject, body), "\n", "\r\n"))

	if err := smtp.SendMail(smtpServer, auth, from, recipients, msg); err != nil {
		return err
	}
	return nil
}
```

mailhogMailerはmailhogコンテナにメールを送るための構造体です。

## Usecase

usecaseディレクトリを作成してください。
usecaseで実装する機能は次のとおりです。

- ユーザーがアクティブかどうか確認する
    - アクティブ（本登録）ならエラー
    - 非アクティブ（仮登録）なら削除して仮登録処理をやり直す
- 本人確認トークンの作成
- メール送信

では実装していきましょう！

usecase/user_usecase.go
```
package usecase

import (
	"context"
	"database/sql"
	"errors"
	"login-example/entity"
	"login-example/mail"
	"login-example/repository"
	"math/rand"
)

type IUserUsecase interface {
	PreRegister(ctx context.Context, email, pw string) (*entity.User, error)
}

type userUsecase struct {
	ur     repository.IUserRepository
	mailer mail.IMailer
}

func NewUserUsecase(ur repository.IUserRepository, mailer mail.IMailer) IUserUsecase {
	return &userUsecase{ur: ur, mailer: mailer}
}

func (uu *userUsecase) PreRegister(ctx context.Context, email, pw string) (*entity.User, error) {
	u, err := uu.ur.GetByEmail(ctx, email)

	// ユーザーが存在しない場合、sql.ErrNoRowsを受け取るはずなので、存在しない場合はそのまま仮登録処理を行う
	if errors.Is(err, sql.ErrNoRows) {
		return uu.preRegister(ctx, email, pw)
		// それ以外のエラーの場合は想定外なのでそのまま返す
	} else if err != nil {
		return nil, err
	}

	// ユーザーがすでにアクティブの場合はエラーを返す
	if u.IsActive() {
		return nil, errors.New("user already active")
	}

	// ユーザーがアクティブではない場合、ユーザーを削除して、再度仮登録処理を行う
	if err := uu.ur.Delete(ctx, u.ID); err != nil {
		return nil, err
	}
	return uu.preRegister(ctx, email, pw)
}

// 仮登録処理を行う
func (uu *userUsecase) preRegister(ctx context.Context, email, pw string) (*entity.User, error) {
	salt := createRandomString(30)
	activeToken := createRandomString(8)

	u := &entity.User{}

	// パスワードのハッシュ化をする
	hashed, err := u.CreateHashedPassword(pw, salt)
	if err != nil {
		return nil, err
	}

	u.Email = email
	u.Salt = salt
	u.Password = hashed
	u.ActivateToken = activeToken
	u.State = entity.UserInactive

	// DBへの仮登録処理を行う
	if err := uu.ur.PreRegister(ctx, u); err != nil {
		return nil, err
	}
	// email宛に、本人確認用のトークンを送信する
	if err := uu.mailer.SendWithActivateToken(email, u.ActivateToken); err != nil {
		return nil, err
	}
	return u, err
}

// lengthの長さのランダムな文字列(a-zA-Z0-9)を作成する
func createRandomString(length uint) string {
	var letterBytes = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
```

はい、usecaseができました。
一旦ここで区切り、残りは次回に行います

# まとめ

今回やったこと

- User構造体を作成
- パスワードのハッシュ化、ソルトについて
- repositoryの作成
- usecaseの作成

また、今のディレクトリはこんな感じです。

```
  ├── .air.toml
  ├── _tools
  │   └── mysql
  │       ├── conf.d
  │       │   └── my.cnf
  │       └── init.d
  │           └── init.sql
  ├── Dockerfile
  ├── db
  │   └── db.go
  ├── docker-compose.yml
+ ├── entity
+ │   └── user.go
  ├── go.mod
  ├── go.sum
+ ├── mail
+ │   └── mailer.go
  ├── main.go
+ ├── repository
+ │   └── user_repository.go
+ └── usecase
+     └── user_usecase.go
```