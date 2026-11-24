// mautrix-teams - A Matrix-Microsoft Teams puppeting bridge.
// Copyright (C) 2026 Sandwich
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
package msteams

import (
	"encoding/base64"
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

var swiftCardRe = regexp.MustCompile(`(?s)<URIObject type="SWIFT\.1"[^>]*>.*?<Swift b64="([^"]+)"`)
var mentionSpanRe = regexp.MustCompile(`(?s)<span[^>]*>(.*?)</span>`)
var boldRe = regexp.MustCompile(`\*\*([^*]+)\*\*`)
var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// RenderSwiftCards converts the URIObject-wrapped Adaptive Card payloads Teams
// sends in RichText/Media_Card messages into a readable plain-text and HTML
// rendering (text blocks, fact sets, containers and open-URL actions; buttons
// become links and Microsoft Teams inputs are dropped). ok is false when the
// content holds no decodable card, so callers can fall back to the raw
// fallback text Teams embedded.
func RenderSwiftCards(content string) (plain, htmlOut string, ok bool) {
	matches := swiftCardRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return "", "", false
	}
	var p, h strings.Builder
	rendered := false
	for _, m := range matches {
		raw, err := base64.StdEncoding.DecodeString(m[1])
		if err != nil {
			continue
		}
		var activity struct {
			Attachments []struct {
				Content json.RawMessage `json:"content"`
			} `json:"attachments"`
		}
		if err := json.Unmarshal(raw, &activity); err != nil {
			continue
		}
		for _, att := range activity.Attachments {
			var card struct {
				Body    []json.RawMessage `json:"body"`
				Actions []struct {
					Type  string `json:"type"`
					Title string `json:"title"`
					URL   string `json:"url"`
				} `json:"actions"`
			}
			if err := json.Unmarshal(att.Content, &card); err != nil {
				continue
			}
			renderCardBlocks(card.Body, &p, &h)
			for _, a := range card.Actions {
				if a.Type != "Action.OpenUrl" || a.URL == "" {
					continue
				}
				title := a.Title
				if title == "" {
					title = a.URL
				}
				p.WriteString("\n🔗 " + title + ": " + a.URL + "\n")
				h.WriteString(`<p>🔗 <a href="` + html.EscapeString(a.URL) + `">` + html.EscapeString(title) + `</a></p>`)
			}
			rendered = true
		}
	}
	if !rendered {
		return "", "", false
	}
	return strings.Trim(p.String(), "\n"), strings.Trim(h.String(), "\n"), true
}

func renderCardBlocks(blocks []json.RawMessage, p, h *strings.Builder) {
	for _, raw := range blocks {
		var block map[string]json.RawMessage
		if json.Unmarshal(raw, &block) != nil {
			continue
		}
		text := rawString(block["text"])
		switch rawString(block["type"]) {
		case "TextBlock":
			weight := rawString(block["weight"])
			if weight == "bolder" {
				p.WriteString(cardTextPlain(text) + "\n")
				h.WriteString("<p><b>" + cardTextHTML(text) + "</b></p>")
			} else {
				p.WriteString(cardTextPlain(text) + "\n")
				h.WriteString("<p>" + cardTextHTML(text) + "</p>")
			}
		case "FactSet":
			var facts []struct {
				Title string `json:"title"`
				Value string `json:"value"`
			}
			if json.Unmarshal(block["facts"], &facts) == nil {
				for _, f := range facts {
					p.WriteString(f.Title + ": " + f.Value + "\n")
					h.WriteString("<p><b>" + html.EscapeString(f.Title) + ":</b> " + cardTextHTML(f.Value) + "</p>")
				}
			}
		case "Container":
			var items []json.RawMessage
			if json.Unmarshal(block["items"], &items) == nil {
				renderCardBlocks(items, p, h)
			}
		case "ColumnSet":
			var columns []struct {
				Items []json.RawMessage `json:"items"`
			}
			if json.Unmarshal(block["columns"], &columns) == nil {
				for _, col := range columns {
					renderCardBlocks(col.Items, p, h)
				}
			}
		}
	}
}

// cardTextPlain strips the markdown-lite markup card text may contain for the
// plain-text rendering.
func cardTextPlain(text string) string {
	text = mentionSpanRe.ReplaceAllString(text, "$1")
	text = boldRe.ReplaceAllString(text, "$1")
	text = mdLinkRe.ReplaceAllString(text, "$1 ($2)")
	return text
}

// cardTextHTML renders a card TextBlock's markdown-lite text as Matrix HTML:
// Skype mention spans become their display name, the rest is escaped, then
// **bold** and [text](url) links are re-applied.
func cardTextHTML(text string) string {
	text = mentionSpanRe.ReplaceAllString(text, "$1")
	text = html.EscapeString(text)
	text = boldRe.ReplaceAllString(text, "<b>$1</b>")
	text = mdLinkRe.ReplaceAllString(text, `<a href="$2">$1</a>`)
	return strings.ReplaceAll(text, "\n", "<br>")
}

func rawString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}
