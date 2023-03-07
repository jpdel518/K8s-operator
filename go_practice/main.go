package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/v47/github"
	"math/rand"
	"mymodule/mypackage"
	"os"
	"time"
)

// 簡易実行
// go run main.go
// ビルド実行
// go build main.go
// ./main

// エンドポイントはpackage名と関数はmainにする必要がある（ファイル名はなんでもOK）
func main() {
	// hello world
	fmt.Println("Hello World")
	greetText := greet("Japanese", "kuro")
	fmt.Println(greetText)
	name := mypackage.GetName()
	fmt.Println(name)

	// using other module package
	client := github.NewClient(nil)
	orgs, _, err := client.Organizations.List(context.Background(), "willnorris", nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for i, org := range orgs {
		fmt.Println(i, *org.Login)
	}

	// interface, struct, embedding
	p := mypackage.Parent{Id: 1, Name: "John"}
	printObject(&p)
	child := mypackage.Child{Parent: p}
	child.SetName("John.Jr")
	printObject(&p)
	printObject(&child)

	// goroutine
	go process()
	fmt.Println("next")
	// sleepしないとgoroutineが実行される前にmain関数が終了してgoroutineも実行されずに終了する
	time.Sleep(2 * time.Second)

	// channel（goroutineから情報を受け取るために使用する）
	ch := make(chan int)
	go func() { ch <- 3 }()
	go func() { ch <- 5 }()
	fmt.Println(<-ch, <-ch)
	// close channel
	close(ch)
	v, ok := <-ch
	fmt.Println(v, ok)

	waitCh := make(chan int)
	go processChannel(waitCh)
	fmt.Println("waiting")
	<-waitCh // receive a message (wait until process is completed)
	fmt.Println("finished")

	// context（複数の関数を跨いで一連の処理の中で「タイムアウト」「キャンセル」「リクエストスコープの値」を伝播する役割）
	rand.Seed(time.Now().UnixNano())
	// contextの初期化
	ctx := context.Background()
	for i := 0; i < 10; i++ { // Call process func 10 times
		err := processCtx(ctx)
		if err != nil {
			fmt.Println(err)
		}
	}
}

// function
func greet(language string, name string) string {
	if language == "Japanese" {
		return fmt.Sprintf("こんにちは %s", name)
	}
	return fmt.Sprintf("Hello %s", name)
}

// interface & function parameter
func printObject(obj mypackage.Object) {
	fmt.Println(obj.GetName())
}

// goroutine
func process() {
	time.Sleep(1 * time.Second)
	fmt.Println("goroutine completed")
}

// channel
func processChannel(ch chan int) {
	fmt.Println("process start")
	time.Sleep(1 * time.Second)

	ch <- 1 // send a message to the channel

	fmt.Println("process finished") // this might not be visible as main() finishes earlier
}

// context
func processCtx(ctx context.Context) error {
	// 3秒過ぎたらtimeoutする子contextを生成
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// 0-5までの整数をランダムに取り出す
	sec := rand.Intn(6)
	fmt.Printf("wait %d sec: ", sec)

	// pseudo process that takes <sec> seconds
	done := make(chan error, 1)
	go func(sec int) {
		time.Sleep(time.Duration(sec) * time.Second)
		done <- nil
	}(sec)

	// 先にdone channelに先に値が入れば「complete」が実行, 先にタイムアウトになればctx.Done()の返り値であるchannelに値が入り「timeout」が実行される
	select {
	case <-done:
		fmt.Println("complete")
		return nil
	case <-ctx.Done():
		fmt.Println("timeout")
		return ctx.Err()
	}
}
