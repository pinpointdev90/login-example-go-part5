package main

import (
	"fmt"
	"login-example/db"
)

func main() {
	db, err := db.NewDB()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	fmt.Println("MySQL接続OK")
}