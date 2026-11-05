package slack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	slackapi "github.com/slack-go/slack"
)

// sharedMessageReplyJSON is the shape conversations.replies returns for a
// message that shares another Slack message: the visible text is short, the
// shared message rides in attachments, and links posted in the message are in
// rich_text blocks.
const sharedMessageReplyJSON = `{
  "type": "message",
  "user": "U0JANE",
  "ts": "1710000100.000200",
  "text": "<@U0BOB> fyi",
  "blocks": [{
    "type": "rich_text",
    "block_id": "b1",
    "elements": [{
      "type": "rich_text_section",
      "elements": [
        {"type": "user", "user_id": "U0BOB"},
        {"type": "text", "text": " fyi see "},
        {"type": "link", "url": "https://example.com/runbook", "text": "runbook"}
      ]
    }, {
      "type": "rich_text_list",
      "style": "bullet",
      "elements": [{
        "type": "rich_text_section",
        "elements": [{"type": "link", "url": "https://example.org/a"}]
      }]
    }, {
      "type": "rich_text_quote",
      "elements": [{"type": "link", "url": "https://example.com/runbook"}]
    }]
  }],
  "attachments": [{
    "id": 1,
    "is_share": true,
    "is_msg_unfurl": true,
    "channel_id": "C0OTHER",
    "channel_name": "ops",
    "ts": "1710000000.000100",
    "author_id": "U0JOHN",
    "author_name": "John Dev",
    "author_link": "https://example.slack.com/team/U0JOHN",
    "from_url": "https://example.slack.com/archives/C0OTHER/p1710000000000100?thread_ts=1709999999.000050&cid=C0OTHER",
    "original_url": "https://example.slack.com/archives/C0OTHER/p1710000000000100",
    "fallback": "[March 9th] John Dev: the *deploy* failed",
    "text": "the *deploy* failed, see <https://example.com/log|the log>",
    "footer": "Thread in #ops",
    "mrkdwn_in": ["text"]
  }, {
    "id": 2,
    "service_name": "Example Docs",
    "title": "Runbook",
    "title_link": "https://example.com/runbook",
    "fallback": "Example Docs: Runbook"
  }]
}`

func decodeSharedMessage(t *testing.T) slackapi.Message {
	t.Helper()
	var message slackapi.Message
	if err := json.Unmarshal([]byte(sharedMessageReplyJSON), &message); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return message
}

func TestSharedContentFromAPIKeepsSharedMessage(t *testing.T) {
	message := decodeSharedMessage(t)
	attachments, links := sharedContentFromAPI(message.Attachments, message.Blocks)
	if len(attachments) != 2 {
		t.Fatalf("attachments = %#v", attachments)
	}
	shared := attachments[0]
	if shared.Kind != AttachmentKindSharedMessage {
		t.Fatalf("kind = %q", shared.Kind)
	}
	if shared.FromURL != "https://example.slack.com/archives/C0OTHER/p1710000000000100?thread_ts=1709999999.000050&cid=C0OTHER" {
		t.Fatalf("from_url = %q", shared.FromURL)
	}
	if shared.Channel != "C0OTHER" || shared.TS != "1710000000.000100" || shared.ThreadTS != "1709999999.000050" {
		t.Fatalf("source = channel %q ts %q thread_ts %q", shared.Channel, shared.TS, shared.ThreadTS)
	}
	if shared.User != "U0JOHN" || shared.AuthorName != "John Dev" {
		t.Fatalf("author = %q %q", shared.User, shared.AuthorName)
	}
	if shared.Text != "the *deploy* failed, see <https://example.com/log|the log>" {
		t.Fatalf("text = %q", shared.Text)
	}
	unfurl := attachments[1]
	if unfurl.Kind != AttachmentKindAttachment || unfurl.Title != "Runbook" || unfurl.TitleLink != "https://example.com/runbook" || unfurl.ServiceName != "Example Docs" {
		t.Fatalf("unfurl = %#v", unfurl)
	}
	if unfurl.Text != "Example Docs: Runbook" {
		t.Fatalf("unfurl text should fall back to fallback, got %q", unfurl.Text)
	}
	want := []string{"https://example.com/runbook", "https://example.org/a"}
	if strings.Join(links, " ") != strings.Join(want, " ") {
		t.Fatalf("links = %#v, want %#v", links, want)
	}
}

func TestSharedContentFromAPIEmptyIsNil(t *testing.T) {
	attachments, links := sharedContentFromAPI(nil, slackapi.Blocks{})
	if attachments != nil || links != nil {
		t.Fatalf("attachments=%#v links=%#v", attachments, links)
	}
}

func TestSharedChannelAndTSFromPermalink(t *testing.T) {
	channel, ts, threadTS := slackPermalinkParts("https://example.slack.com/archives/C0OTHER/p1710000000000100?thread_ts=1709999999.000050&cid=C0OTHER")
	if channel != "C0OTHER" || ts != "1710000000.000100" || threadTS != "1709999999.000050" {
		t.Fatalf("parts = %q %q %q", channel, ts, threadTS)
	}
	if c, s, _ := slackPermalinkParts("https://example.com/runbook"); c != "" || s != "" {
		t.Fatalf("non-slack url parsed as %q %q", c, s)
	}
}

func sharedFixture() []MessageAttachment {
	return []MessageAttachment{{
		Kind:    AttachmentKindSharedMessage,
		FromURL: "https://example.slack.com/archives/C0OTHER/p1710000000000100",
		Channel: "C0OTHER",
		TS:      "1710000000.000100",
		User:    "U0JOHN",
		Text:    "the *deploy* failed",
	}}
}

func TestReadOperationsReturnSharedContent(t *testing.T) {
	message := ThreadMessage{TS: "1710000100.000200", User: "U0JANE", Text: "<@U0BOB> fyi", Attachments: sharedFixture(), Links: []string{"https://example.com/runbook"}}
	factory := &capturingFactory{
		clients: map[string]*fakeClient{
			"user_token": {
				thread:         []ThreadMessage{message},
				messageHistory: MessageHistory{Messages: []ThreadMessage{message}},
				searchMessages: []SearchMessage{{Channel: "C1", TS: "1710000100.000200", User: "U0JANE", Text: "<@U0BOB> fyi", Attachments: sharedFixture(), Links: []string{"https://example.com/runbook"}}},
				searchTotal:    1,
			},
		},
	}
	plugin := testPlugin(factory)

	thread := plugintest.RunOK[ThreadResult](t, plugin, OperationThread, map[string]any{"channel": "C1", "ts": "1710000100.000200", "text_format": "both"})
	got := thread.Messages[0]
	if len(got.Attachments) != 1 || got.Attachments[0].Text != "the **deploy** failed" || got.Attachments[0].TextMrkdwn != "the *deploy* failed" {
		t.Fatalf("thread attachments = %#v", got.Attachments)
	}
	if got.Attachments[0].FromURL == "" || got.Attachments[0].User != "U0JOHN" || got.Attachments[0].Channel != "C0OTHER" || got.Attachments[0].TS != "1710000000.000100" {
		t.Fatalf("thread attachment source = %#v", got.Attachments[0])
	}
	if len(got.Links) != 1 || got.Links[0] != "https://example.com/runbook" {
		t.Fatalf("thread links = %#v", got.Links)
	}

	hist := plugintest.RunOK[MessageListResult](t, plugin, OperationMessageList, map[string]any{"channel": "C1"})
	if len(hist.Messages[0].Attachments) != 1 || !strings.Contains(hist.Messages[0].Attachments[0].Text, "**deploy**") || hist.Messages[0].Attachments[0].TextMrkdwn != "" || len(hist.Messages[0].Links) != 1 {
		t.Fatalf("message.list shared content = %#v", hist.Messages[0])
	}

	search := plugintest.RunOK[SearchResult](t, plugin, OperationSearch, map[string]any{"query": "fyi"})
	if len(search.Messages[0].Attachments) != 1 || search.Messages[0].Attachments[0].FromURL == "" || len(search.Messages[0].Links) != 1 {
		t.Fatalf("search shared content = %#v", search.Messages[0])
	}
}

func TestReadOperationOutputSchemasDeclareSharedContent(t *testing.T) {
	want := map[string]bool{OperationThread: false, OperationMessageList: false, OperationSearch: false}
	for _, operation := range Manifest().Operations {
		if _, ok := want[operation.Name]; !ok {
			continue
		}
		schema := string(operation.Output)
		for _, field := range []string{`"attachments"`, `"links"`, `"from_url"`, `"author_name"`} {
			if !strings.Contains(schema, field) {
				t.Fatalf("%s output schema lacks %s: %s", operation.Name, field, schema)
			}
		}
		want[operation.Name] = true
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("operation %s not in manifest", name)
		}
	}
}
