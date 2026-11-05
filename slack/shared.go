package slack

import (
	"net/url"
	"strings"

	slackapi "github.com/slack-go/slack"
)

const (
	// AttachmentKindSharedMessage marks an attachment that carries another
	// Slack message (a share, a forward or a pasted message permalink).
	AttachmentKindSharedMessage = "shared_message"
	// AttachmentKindAttachment is any other attachment: a link unfurl, a bot
	// attachment.
	AttachmentKindAttachment = "attachment"
)

// MessageAttachment is a message's attachment as the reader needs it: a
// shared or forwarded Slack message with its source and author, or a link
// unfurl. Without it a message that shares another one reads as its short
// caption only ("fyi").
type MessageAttachment struct {
	Kind        string `json:"kind" jsonschema:"description=shared_message when the attachment carries another Slack message; attachment otherwise,enum=shared_message,enum=attachment"`
	FromURL     string `json:"from_url,omitempty" jsonschema:"description=URL the attachment was made from; for a shared message its Slack permalink"`
	OriginalURL string `json:"original_url,omitempty"`
	Channel     string `json:"channel,omitempty" jsonschema:"description=Channel ID of the shared message"`
	TS          string `json:"ts,omitempty" jsonschema:"description=Timestamp of the shared message"`
	ThreadTS    string `json:"thread_ts,omitempty" jsonschema:"description=Thread the shared message belongs to"`
	User        string `json:"user,omitempty" jsonschema:"description=Author user ID of the shared message"`
	AuthorName  string `json:"author_name,omitempty"`
	AuthorLink  string `json:"author_link,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	Title       string `json:"title,omitempty"`
	TitleLink   string `json:"title_link,omitempty"`
	Pretext     string `json:"pretext,omitempty"`
	// Text follows the message's text_format, like the message text.
	Text       string `json:"text,omitempty"`
	TextMrkdwn string `json:"text_mrkdwn,omitempty"`
	Footer     string `json:"footer,omitempty"`
}

// sharedContentFromAPI keeps what a message carries beside its text: the
// attachments (shared messages, unfurls) and every link in its rich_text
// blocks, de-duplicated in order of appearance.
func sharedContentFromAPI(attachments []slackapi.Attachment, blocks slackapi.Blocks) ([]MessageAttachment, []string) {
	var out []MessageAttachment
	for _, attachment := range attachments {
		item := MessageAttachment{
			Kind:        AttachmentKindAttachment,
			FromURL:     strings.TrimSpace(attachment.FromURL),
			OriginalURL: strings.TrimSpace(attachment.OriginalURL),
			User:        strings.TrimSpace(attachment.AuthorID),
			AuthorName:  strings.TrimSpace(attachment.AuthorName),
			AuthorLink:  strings.TrimSpace(attachment.AuthorLink),
			ServiceName: strings.TrimSpace(attachment.ServiceName),
			Title:       strings.TrimSpace(attachment.Title),
			TitleLink:   strings.TrimSpace(attachment.TitleLink),
			Pretext:     strings.TrimSpace(attachment.Pretext),
			Text:        strings.TrimSpace(firstNonEmpty(attachment.Text, attachment.Fallback)),
			Footer:      strings.TrimSpace(attachment.Footer),
		}
		channel, ts, threadTS := slackPermalinkParts(firstNonEmpty(item.FromURL, item.OriginalURL))
		if channel != "" {
			item.Kind = AttachmentKindSharedMessage
			item.Channel, item.TS, item.ThreadTS = channel, ts, threadTS
		}
		if attachmentTS := normalizeSlackTimestamp(attachment.Ts.String()); attachmentTS != "" && item.Kind == AttachmentKindSharedMessage {
			item.TS = attachmentTS
		}
		out = append(out, item)
	}
	links := blockLinks(blocks)
	for _, attachment := range attachments {
		links = appendUniqueLinks(links, blockLinks(attachment.Blocks)...)
	}
	if len(links) == 0 {
		links = nil
	}
	return out, links
}

// renderAttachmentText applies a message's text format to its attachments.
func renderAttachmentText(attachments []MessageAttachment, format textFormat) {
	for i := range attachments {
		raw := attachments[i].Text
		switch format {
		case textFormatMrkdwn:
			attachments[i].Text, attachments[i].TextMrkdwn = raw, ""
		case textFormatBoth:
			attachments[i].Text, attachments[i].TextMrkdwn = MrkdwnToMarkdown(raw), raw
		default:
			attachments[i].Text, attachments[i].TextMrkdwn = MrkdwnToMarkdown(raw), ""
		}
	}
}

// slackPermalinkParts reads channel, ts and thread_ts from a Slack message
// permalink (https://<workspace>.slack.com/archives/<channel>/p<ts>). Any
// other URL yields empty parts.
func slackPermalinkParts(raw string) (channel, ts, threadTS string) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", ""
	}
	parts := splitPath(parsed.Path)
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "archives" && strings.HasPrefix(parts[i+2], "p") {
			return parts[i+1], normalizeSlackTimestamp(parts[i+2]), normalizeSlackTimestamp(parsed.Query().Get("thread_ts"))
		}
	}
	return "", "", ""
}

func blockLinks(blocks slackapi.Blocks) []string {
	var links []string
	for _, block := range blocks.BlockSet {
		richText, ok := block.(*slackapi.RichTextBlock)
		if !ok || richText == nil {
			continue
		}
		links = richTextElementLinks(links, richText.Elements)
	}
	return links
}

func richTextElementLinks(links []string, elements []slackapi.RichTextElement) []string {
	for _, element := range elements {
		switch e := element.(type) {
		case *slackapi.RichTextSection:
			links = richTextSectionLinks(links, e.Elements)
		case *slackapi.RichTextList:
			links = richTextElementLinks(links, e.Elements)
		case *slackapi.RichTextQuote:
			links = richTextSectionLinks(links, e.Elements)
		case *slackapi.RichTextPreformatted:
			links = richTextSectionLinks(links, e.Elements)
		}
	}
	return links
}

func richTextSectionLinks(links []string, elements []slackapi.RichTextSectionElement) []string {
	for _, element := range elements {
		if link, ok := element.(*slackapi.RichTextSectionLinkElement); ok && link != nil {
			links = appendUniqueLinks(links, link.URL)
		}
	}
	return links
}

func appendUniqueLinks(links []string, candidates ...string) []string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		seen := false
		for _, existing := range links {
			if existing == candidate {
				seen = true
				break
			}
		}
		if !seen {
			links = append(links, candidate)
		}
	}
	return links
}
