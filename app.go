package main

import (
	"context"
	"fmt"
)

// App はWailsアプリケーションの仮実装
// 将来的にinternal/app配下のハンドラーに分割する
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, from BMS ELSA!", name)
}
