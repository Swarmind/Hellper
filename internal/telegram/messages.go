package telegram

const TokenInputMessage = "Please enter the token for %s endpoint:"

const ConfigMessage = "You can configure usage of external sessions for voice and images using buttons below:"

const EndpointUsingMessage = "Endpoint %s selected"
const EndpointSelectMessage = "Select endpoint for %s type messages using buttons below:"

const ModelUsingMessage = "Model %s selected"
const ModelSelectMessage = "Select model for %s type messages using buttons below:"

const ClearMessage = "Message history cleared"

const EndMessage = "I will stop replying to your messages. Tag me in chat to continue the conversation"
const EndInPrivateMessage = "Has no effect in private chat"

const LogoutMessage = "Logout from endpoint %s successful"

const UsageTokens = `	Completion: %d
	Prompt: %d
	Total: %d
`
const UsageTimings = `	Prompt processing: %.1fms (%.1ft/s)
	Token generation: %.1fms (%.1ft/s)
`
const UsageMessage = `Global usage:
%s
Session usage:
%s
Last usage:
%s
`
