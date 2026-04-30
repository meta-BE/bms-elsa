// ISO 8601 (例: "2024-01-15T00:00:00Z") から YYYY-MM-DD 部分のみ抽出する。
// new Date() 経由のフォーマットだとローカルタイムゾーン変換で日付がずれるため、
// 文字列の先頭10文字を採用して UTC 日付をそのまま表示する。
export function formatDateYMD(iso: string): string {
  return iso.slice(0, 10)
}
