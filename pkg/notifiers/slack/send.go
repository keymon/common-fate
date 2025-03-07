package slacknotifier

import (
	"context"

	"github.com/common-fate/common-fate/pkg/storage"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

// SendMessageBlocks is a utility for sending DMs to users by ID
//
// The message is in Slack message block format.
// The summary must be plaintext and is used as the fallback
// message in Slack notifications.
// The mesage is addressed to userEmail (therefore this is for DMs only, no webhook support)
func SendMessageBlocks(ctx context.Context, slackClient *slack.Client, userEmail string, message slack.Message, summary string) (timestamp string, error error) {
	u, err := slackClient.GetUserByEmailContext(ctx, userEmail)
	if err != nil {
		return "", err
	}

	result, _, _, err := slackClient.OpenConversationContext(ctx, &slack.OpenConversationParameters{
		Users: []string{u.ID},
	})
	if err != nil {
		return "", err
	}
	_, ts, _, err := slackClient.SendMessageContext(ctx, result.Conversation.ID, slack.MsgOptionBlocks(message.Blocks.BlockSet...), slack.MsgOptionText(summary, false))

	if err != nil {
		return "", err
	} else {
		return ts, nil
	}
}

// SendMessage is a utility for sending DMs to users by ID
//
// The message may be markdown formatted. The summary must be plaintext and is used as the fallback
// message in Slack notifications.
// Used for brief messages to requestors like:
// `Your access to *%s* has now expired. If you still need access you can send another request using Common Fate`
func SendMessage(ctx context.Context, slackClient *slack.Client, userEmail, message, summary string, accessory *slack.Accessory) (timestamp string, error error) {
	u, err := slackClient.GetUserByEmailContext(ctx, userEmail)
	if err != nil {
		return "", err
	}
	result, _, _, err := slackClient.OpenConversationContext(ctx, &slack.OpenConversationParameters{
		Users: []string{u.ID},
	})
	if err != nil {
		return "", err
	}
	block := slack.NewTextBlockObject("mrkdwn", message, false, false)
	msgBlock := slack.NewSectionBlock(block, nil, accessory)

	_, ts, _, err := slackClient.SendMessageContext(ctx, result.Conversation.ID, slack.MsgOptionBlocks(msgBlock), slack.MsgOptionText(summary, false))
	if err != nil {
		return "", err
	} else {
		return ts, nil
	}
}

// SendDMWithLogOnError attempts to fetch a user from cognito to get their email, then tries to send them a message in slack, this is used to send PMs to users for updated access details
//
// This will log any errors and continue
func (n *SlackNotifier) SendDMWithLogOnError(ctx context.Context, log *zap.SugaredLogger, userId, msg, fallback string) (timestamp string) {
	userQuery := storage.GetUser{ID: userId}
	_, err := n.DB.Query(ctx, &userQuery)
	if err != nil {
		log.Errorw("Failed to fetch user by id while trying to send message in slack", "uid", userId, "error", err)
		return
	}

	if n.directMessageClient != nil {
		timestamp, err = SendMessage(ctx, n.directMessageClient.client, userQuery.Result.Email, msg, fallback, nil)
		if err != nil {
			log.Errorw("Failed to send direct message", "email", userQuery.Result.Email, "msg", msg, "error", err)
		}
		return timestamp
	}
	return ""
}
