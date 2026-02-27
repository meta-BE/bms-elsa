package gateway

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/meta-BE/bms-elsa/internal/port"
)

// ParseLR2IRResponse はLR2IRのHTMLレスポンス（UTF-8変換済み）をパースする
func ParseLR2IRResponse(body string) (*port.IRResponse, error) {
	if strings.Contains(body, "この曲は登録されていません。") {
		return &port.IRResponse{Registered: false}, nil
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp := &port.IRResponse{Registered: true}

	// h4=Genre, h1=Title, h2=Artist を取得
	resp.Genre = textContent(findFirstElement(doc, "h4"))
	resp.Title = textContent(findFirstElement(doc, "h1"))
	resp.Artist = textContent(findFirstElement(doc, "h2"))

	// "情報" セクションの <table> を見つけてパース
	infoTable := findInfoTable(doc)
	if infoTable != nil {
		parseInfoTable(infoTable, resp)
	}

	return resp, nil
}

// findFirstElement はDOMツリーから指定タグの最初の要素を返す
func findFirstElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirstElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

// textContent はノード配下のすべてのテキストを連結して返す
func textContent(n *html.Node) string {
	if n == nil {
		return ""
	}
	var sb strings.Builder
	collectText(n, &sb)
	return strings.TrimSpace(sb.String())
}

func collectText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectText(c, sb)
	}
}

// findInfoTable は <h3>情報</h3> 直後の <table> を返す
func findInfoTable(doc *html.Node) *html.Node {
	var found bool
	var result *html.Node
	walkNodes(doc, func(n *html.Node) bool {
		if result != nil {
			return false
		}
		if n.Type == html.ElementNode && n.Data == "h3" {
			t := textContent(n)
			if strings.Contains(t, "情報") {
				found = true
				return true
			}
		}
		if found && n.Type == html.ElementNode && n.Data == "table" {
			result = n
			return false
		}
		return true
	})
	return result
}

// walkNodes はDOMツリーを深さ優先で走査する。fnがfalseを返すと走査を中止する。
func walkNodes(n *html.Node, fn func(*html.Node) bool) bool {
	if !fn(n) {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !walkNodes(c, fn) {
			return false
		}
	}
	return true
}

// parseInfoTable は情報テーブルの各行をパースしてrespに格納する
func parseInfoTable(table *html.Node, resp *port.IRResponse) {
	rows := findChildElements(table, "tr")
	for _, row := range rows {
		ths := findChildElements(row, "th")
		tds := findChildElements(row, "td")

		if len(ths) == 0 {
			continue
		}

		firstTH := textContent(ths[0])

		switch {
		case firstTH == "BPM":
			// 1行目: BPM / レベル / 鍵盤数 / 判定ランク
			for i, th := range ths {
				if i >= len(tds) {
					break
				}
				label := textContent(th)
				value := strings.TrimSpace(textContent(tds[i]))
				switch label {
				case "BPM":
					resp.BPM = value
				case "レベル":
					resp.Level = value
				case "鍵盤数":
					resp.Keys = value
				case "判定ランク":
					resp.JudgeRank = value
				}
			}

		case firstTH == "タグ":
			if len(tds) > 0 {
				resp.Tags = parseTagsFromTD(tds[0])
			}

		case firstTH == "本体URL":
			if len(tds) > 0 {
				resp.BodyURL = extractHref(tds[0])
			}

		case firstTH == "差分URL":
			if len(tds) > 0 {
				resp.DiffURL = extractHref(tds[0])
			}

		case firstTH == "備考":
			if len(tds) > 0 {
				resp.Notes = strings.TrimSpace(textContent(tds[0]))
			}
		}
	}
}

// findChildElements は子孫から指定タグの要素をすべて返す（再帰的に探索）
func findChildElements(n *html.Node, tag string) []*html.Node {
	var result []*html.Node
	// trを探す場合はtbodyの中にいる可能性があるため再帰的に探索
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == tag {
				result = append(result, c)
			} else if c.Type == html.ElementNode {
				// tbodyなどの中間要素を透過的に走査
				walk(c)
			}
		}
	}
	walk(n)
	return result
}

// parseTagsFromTD は <td> 内の <a> タグからタグ名を抽出する（keyword=が空のものは除外）
func parseTagsFromTD(td *html.Node) []string {
	var tags []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := getAttr(n, "href")
			if href != "" && hasNonEmptyKeyword(href) {
				text := strings.TrimSpace(textContent(n))
				if text != "" {
					tags = append(tags, text)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(td)
	return tags
}

// hasNonEmptyKeyword はURLのkeywordパラメータが空でないかを判定する
func hasNonEmptyKeyword(href string) bool {
	u, err := url.Parse(html.UnescapeString(href))
	if err != nil {
		return false
	}
	keyword := u.Query().Get("keyword")
	return keyword != ""
}

// extractHref は <td> 内の最初の <a> タグの href を返す（HTMLエンティティをデコード）
func extractHref(td *html.Node) string {
	var href string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if href != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			href = html.UnescapeString(getAttr(n, "href"))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(td)
	return href
}

// getAttr はノードから指定属性の値を返す
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
