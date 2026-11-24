package msteams_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"go.mau.fi/mautrix-teams/pkg/msteams"
)

// Fixture mirrors a real captured RichText/Media_Card message (URIObject with
// base64 Swift attachment containing an AdaptiveCard).
func cardContent(t *testing.T, card map[string]any) string {
	t.Helper()
	activity := map[string]any{
		"attachments": []map[string]any{{"content": card}},
	}
	raw, err := json.Marshal(activity)
	if err != nil {
		t.Fatal(err)
	}
	return `<URIObject type="SWIFT.1" url_thumbnail="https://urlp.asm.skype.com/x">Card - access it on ` +
		`<a href="https://go.skype.com/cards.unsupported">https://go.skype.com/cards.unsupported</a>. <Title>Card</Title>` +
		`<Swift b64="` + base64.StdEncoding.EncodeToString(raw) + `" /><Description /></URIObject>`
}

func TestRenderSwiftCards(t *testing.T) {
	content := cardContent(t, map[string]any{
		"type": "AdaptiveCard",
		"body": []map[string]any{
			{"type": "TextBlock", "text": "✅ Release 12.21: all clear", "weight": "bolder", "wrap": true},
			{"type": "TextBlock", "text": "Release **12.21** is wrapped up. Thanks\nall!", "wrap": true},
			{"type": "TextBlock", "text": `<span contenteditable="false" itemscope="" itemtype="http://schema.skype.com/Mention" itemid="0">Thomas Wagner</span>, release coordinator`, "wrap": true},
			{"type": "FactSet", "facts": []map[string]any{{"title": "Env", "value": "prod"}}},
			{"type": "Container", "items": []map[string]any{{"type": "TextBlock", "text": "nested item"}}},
		},
		"actions": []map[string]any{{"type": "Action.OpenUrl", "title": "Open release 12.21", "url": "https://example.com/release"}},
	})
	plain, htmlOut, ok := msteams.RenderSwiftCards(content)
	if !ok {
		t.Fatal("expected card to be rendered")
	}
	if strings.Contains(plain, "cards.unsupported") {
		t.Error("fallback text should be replaced")
	}
	for _, want := range []string{
		"✅ Release 12.21: all clear",
		"Release 12.21 is wrapped up",
		"Thomas Wagner, release coordinator",
		"Env: prod",
		"nested item",
		"Open release 12.21: https://example.com/release",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain missing %q:\n%s", want, plain)
		}
	}
	for _, want := range []string{
		"<b>✅ Release 12.21: all clear</b>",
		"Release <b>12.21</b> is wrapped up. Thanks<br>all!",
		"Thomas Wagner, release coordinator",
		`<a href="https://example.com/release">Open release 12.21</a>`,
	} {
		if !strings.Contains(htmlOut, want) {
			t.Errorf("html missing %q:\n%s", want, htmlOut)
		}
	}
	if strings.Contains(htmlOut, "itemtype=") {
		t.Error("mention span markup should not leak into html")
	}
}

func TestRenderSwiftCardsNoCard(t *testing.T) {
	if _, _, ok := msteams.RenderSwiftCards("<p>plain message</p>"); ok {
		t.Error("plain content should not be treated as a card")
	}
	if _, _, ok := msteams.RenderSwiftCards(`<URIObject type="SWIFT.1"><Swift b64="!!!notbase64!!!" /></URIObject>`); ok {
		t.Error("undecodable card should not render")
	}
}
