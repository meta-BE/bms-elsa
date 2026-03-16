package port

// Logger は操作ログの書き込みインターフェース
type Logger interface {
	Log(message string)
}

// NopLogger は何もしないLogger（ログ初期化失敗時のフォールバック）
type NopLogger struct{}

func (NopLogger) Log(string) {}
