# internal/telegram/database.go  
## Package: telegram  
  
### Imports:  
- database/sql  
- github.com/go-telegram/bot/models  
  
### External Data, Input Sources:  
- Database: The code interacts with a database using the `database/sql` package. It uses a table named `tg_session` to store user data.  
  
### TODOs:  
- None found in the provided code.  
  
### Summary:  
The code defines a `Service` struct and provides methods for managing user data in a Telegram bot. The `CreateTables` method creates a table named `tg_session` if it doesn't exist. The `GetUser` method retrieves user data from the database, and if the user doesn't exist, it inserts a new entry into the table. The `SetChatData` method updates the chat data for a user, including chat ID, thread ID, chat type, and forum status. The `SetInDialogState` method updates the in-dialog state for a user, and the `SetAwaitingToken` method updates the awaiting token message ID for a user.  
  
The code uses SQL queries to interact with the database and provides methods for managing user data, such as chat ID, thread ID, chat type, forum status, in-dialog state, and awaiting token message ID.  
  
# internal/telegram/handlers.go  
```  
package telegram  
  
import (  
	"context"  
	"database/sql"  
	"fmt"  
	"hellper/internal/ai"  
	"log"  
	"slices"  
	"strconv"  
	"strings"  
  
	"github.com/go-telegram/bot"  
	"github.com/go-telegram/bot/models"  
)  
  
const (  
	TokenInputPrompt = "Please enter the token for %s endpoint:"  
	EndpointNotFoundMessage = "Endpoint with that name not found"  
	EndpointUsingMessage = "Endpoint %s selected"  
	EndpointSelectMessage = "Select endpoint using keyboard below"  
  
	ModelNotFoundMessage = "Model with that name not found"  
	ModelUsingMessage = "Model %s selected\n\nYou can start the conversation"  
	ModelSelectMessage = "Select model using keyboard below"  
  
	ClearMessage = "Message history cleared"  
  
	EndMessage = "I will stop replying to your messages. Tag me in chat to continue the conversation"  
	EndInPrivateMessage = "Has no effect in private chat"  
  
	LogoutMessage = "Logout from endpoint %s successful"  
  
	UsageMessage = `Global usage:  
	Completion: %d  
	Prompt: %d  
	Total: %d  
  
	Prompt processing: %.1fms (%.1ft/s)  
	Token generation: %.1fms (%.1ft/s)  
  
Session usage:  
	Completion: %d  
	Prompt: %d  
	Total: %d  
  
	Prompt processing: %.1fms (%.1ft/s)  
	Token generation: %.1fms (%.1ft/s)  
  
Last usage:  
	Completion: %d  
	Prompt: %d  
	Total: %d  
  
	Prompt processing: %.1fms (%.1ft/s)  
	Token generation: %.1fms (%.1ft/s)`  
  
)  
  
func (s *Service) EndpointHandler(ctx context.Context, b *bot.Bot, update *models.Update) {  
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,  
		update.Message.Chat.ID,  
		update.Message.MessageThreadID,  
		update.Message.Chat.IsForum,  
		update.Message.Chat.Type  
	defer DeleteMessageLog("EndpointHandler", s.Bot, s.Ctx, chatId, update.Message.ID)  
  
	response := CreateResponseMessageParams(chatId, threadId, isForum)  
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {  
		response.Text = fmt.Sprintf("SetChatData error: %v", err)  
		SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)  
		return  
	}  
  
	endpoints, err := s.AI.GetEndpoints()  
	if err != nil {  
		response.Text = fmt.Sprintf("Database.GetEndpoints error: %v", err)  
		SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)  
		return  
	}  
  
	endpointName := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/endpoint"))  
	if endpointName != "" {  
		var endpoint *ai.Endpoint  
		for _, i := range endpoints {  
			if i.Name == endpointName {  
				endpoint = &i  
				break  
			}  
		}  
		if endpoint == nil {  
			response.Text = EndpointNotFoundMessage  
			SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)  
			return  
		}  
  
		if err := s.AI.UpdateEndpoint(userId, &endpoint.ID); err != nil {  
			response.Text = fmt.Sprintf("AI.UpdateEndpoint error: %v", err)  
			SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)  
			return  
		}  
		if err := s.AI.UpdateModel(userId, nil); err != nil {  
			response.Text = fmt.Sprintf("AI.UpdateModel error: %v", err)  
			SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)  
			return  
		}  
		s.AI.DropHandler(userId)  
  
		response.Text = fmt.Sprintf(EndpointUsingMessage, endpointName)  
		SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)  
		return  
	}  
  
	response.Text = EndpointSelectMessage  
	response.ReplyMarkup = CreateEndpointsMarkup(endpoints)  
	SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)  
}  
  
func (s *Service) EndpointCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {  
	if _, err := s.Bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{  
		CallbackQueryID: update.CallbackQuery.ID,  
		ShowAlert:       false,  
	}); err != nil {  
		log.Printf("Bot.AnswerCallbackQuery error: %v", err)  
		return  
	}  
  
	userId, chatId, threadId, isForum, chatType := update.CallbackQuery.From.ID,  
		update.CallbackQuery.Message.Message.Chat.ID,  
		update.CallbackQuery.Message.Message.MessageThreadID,  
		update.CallbackQuery.Message.Message.Chat.IsForum,  
		update.CallbackQuery.Message.Message.Chat.Type  
	defer DeleteMessageLog("EndpointCallbackHandler", s.Bot, s.Ctx,  
		chatId, update.CallbackQuery.Message.Message.ID)  
  
	response := CreateResponseMessageParams(chatId, threadId, isForum)  
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {  
		response.Text = fmt.Sprintf("SetChatData error: %v", err)  
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)  
		return  
	}  
  
	endpointId, err := strconv.ParseInt(  
		strings.TrimSpace(strings.TrimPrefix(update.CallbackQuery.Data, "endpoint_")),  
		10, 64,  
	)  
	if err != nil {  
		response.Text = fmt.Sprintf("strconv.ParseInt error: %v", err)  
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)  
		return  
	}  
  
	endpoints, err := s.AI.GetEndpoints()  
	if err != nil {  
		response.Text = fmt.Sprintf("Database.GetEndpoints error: %v", err)  
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)  
		return  
	}  
	var endpoint *ai.Endpoint  
	for _, i := range endpoints {  
		if i.ID == endpointId {  
			endpoint = &i  
			break  
		}  
	}  
	if endpoint == nil {  
		response.Text = EndpointNotFoundMessage  
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)  
		return  
	}  
  
	if err := s.AI.UpdateEndpoint(userId, &endpoint.ID); err != nil {  
		response.Text = fmt.Sprintf("AI.UpdateEndpoint error: %v", err)  
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)  
		return  
	}  
	if err := s.AI.UpdateModel(userId, nil); err != nil {  
		response.Text = fmt.Sprintf("AI.UpdateModel error: %v", err)  
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)  
		return  
	}  
	s.AI.DropHandler(userId)  
  
	response.Text = fmt.Sprintf(EndpointUsingMessage, endpoint.Name)  
	SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)  
}  
  
func (s *Service) ModelHandler(ctx context.Context, b *bot.Bot, update *models.Update) {  
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,  
		update.Message.Chat.ID,  
		update.Message.MessageThreadID,  
		update.Message.Chat.IsForum,  
		update.Message.Chat.Type  
	response := CreateResponseMessageParams(chatId, threadId, isForum)  
	defer DeleteMessageLog("ModelHandler", s.Bot, s.Ctx, chatId, update.Message.ID)  
  
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {  
		response.Text = fmt.Sprintf("SetChatData error: %v", err)  
		SendResponseLog("  
  
# internal/telegram/root_handler.go  
## telegram  
  
### Imports  
```  
import (  
	"context"  
	"database/sql"  
	"fmt"  
	"log"  
	"strings"  
  
	"github.com/go-telegram/bot"  
	"github.com/go-telegram/bot/models"  
)  
```  
  
### External Data, Input Sources  
- Database: Used for storing user data, session information, and model endpoints.  
- Telegram Bot API: Used for sending and receiving messages, as well as managing chat actions.  
  
### TODOs  
- Implement the `chainModelChoice` function to handle model selection based on user input.  
- Implement the `chainTokenInput` function to handle token input based on user input.  
  
### Summary of Major Code Parts  
1. `RootHandler` function:  
   - This function is the main handler for incoming messages from the Telegram bot.  
   - It extracts user and chat information from the update object.  
   - It checks if the chat is private and if the user is in a dialog state.  
   - It sets chat data and retrieves the user's session information.  
   - If the user is awaiting a token, it inserts the token into the database and updates the user's state.  
   - If the user has a valid token, it retrieves the model list and presents it to the user for selection.  
   - Once a model is selected, it performs inference using the AI model and sends the response back to the user.  
  
2. `CreateResponseMessageParams` function:  
   - This function creates a new response message object with the necessary parameters for sending a message to the Telegram bot.  
  
3. `SetChatData` function:  
   - This function stores chat data for the current user, including chat ID, thread ID, forum status, and chat type.  
  
4. `SetInDialogState` function:  
   - This function sets the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
5. `GetSession` function:  
   - This function retrieves the user's session information from the database.  
  
6. `GetEndpoints` function:  
   - This function retrieves a list of available AI model endpoints from the database.  
  
7. `InsertToken` function:  
   - This function inserts a new token into the database for the given user and endpoint.  
  
8. `GetToken` function:  
   - This function retrieves a token for the given user and endpoint from the database.  
  
9. `GetModelsList` function:  
   - This function retrieves a list of available models for the given endpoint and token.  
  
10. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
11. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
12. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
13. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
14. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
15. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
16. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
17. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
18. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
19. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
20. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
21. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
22. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
23. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
24. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
25. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
26. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
27. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
28. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
29. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
30. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
31. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
32. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
33. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
34. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
35. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
36. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
37. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
38. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
39. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
40. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
41. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
42. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
43. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
44. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
45. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
46. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
47. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
48. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
49. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
50. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
51. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
52. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
53. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
54. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
55. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
56. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
57. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
58. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
59. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
60. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
61. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
62. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
63. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
64. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
65. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
66. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
67. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
68. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
69. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
70. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
71. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
72. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
73. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
74. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
75. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
76. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
77. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
78. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
79. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
80. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
81. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
82. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
83. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
84. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
85. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
86. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
87. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
88. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
89. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
90. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
91. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
92. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
93. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
94. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
95. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
96. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
97. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
98. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
99. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
100. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
101. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
102. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
103. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
104. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
105. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
106. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
107. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
108. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
109. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
110. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
111. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
112. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
113. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
114. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
115. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
116. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
117. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
118. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
119. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
120. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
121. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
122. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
123. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
124. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
125. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
126. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
127. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
128. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
129. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
130. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
131. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
132. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
133. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
134. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
135. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
136. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
137. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
138. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
139. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
140. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
141. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
142. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
143. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
144. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
145. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
146. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
147. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
148. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
149. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
150. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
151. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
152. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
153. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
154. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
155. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
156. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
157. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
158. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
159. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
160. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
161. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
162. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
163. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
164. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
165. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
166. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
167. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
168. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
169. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
170. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
171. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
172. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
173. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
174. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
175. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
176. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
177. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
178. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
179. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
180. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
181. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
182. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
183. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
184. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot.  
  
185. `GetModelsList` function:  
    - This function retrieves a list of available models for the given endpoint and token.  
  
186. `Inference` function:  
    - This function performs inference using the selected AI model and returns the generated text.  
  
187. `SendResponseLog` function:  
    - This function sends a response message to the Telegram bot and logs the response.  
  
188. `DeleteMessageLog` function:  
    - This function deletes a message from the Telegram chat and logs the deletion.  
  
189. `SendChatActionLog` function:  
    - This function sends a chat action to the Telegram bot and logs the action.  
  
190. `EndpointSelectMessage` and `ModelSelectMessage`:  
    - These are predefined messages that are sent to the user when they need to select an endpoint or model.  
  
191. `CreateEndpointsMarkup` and `CreateModelsMarkup`:  
    - These functions create reply markup objects for the endpoint and model selection messages.  
  
192. `EmptyTokenMessage`:  
    - This is a predefined message that is sent to the user when they have not provided a token.  
  
193. `Username`:  
    - This variable stores the bot's username, which is used to identify the bot in the chat.  
  
194. `chainModelChoice` and `chainTokenInput`:  
    - These functions are responsible for handling model and token input based on user input. They are not implemented in the provided code and need to be implemented separately.  
  
195. `AI`:  
    - This variable represents the AI model that is used for inference.  
  
196. `Bot`:  
    - This variable represents the Telegram bot object that is used for sending and receiving messages.  
  
197. `Ctx`:  
    - This variable represents the context for the bot's operations.  
  
198. `GetUser` function:  
    - This function retrieves user data from the database based on the user ID.  
  
199. `SetAwaitingToken` function:  
    - This function updates the user's state to indicate that they are awaiting a token.  
  
200. `SetInDialogState` function:  
    - This function updates the user's dialog state to true, indicating that they are currently in a dialog with the bot  
  
# internal/telegram/service.go  
## Package: telegram  
  
### Imports:  
  
- context  
- hellper/internal/ai  
- hellper/internal/database  
- os  
- os/signal  
- github.com/go-telegram/bot  
  
### External Data, Input Sources:  
  
- Database: database.Handler  
- AI Service: ai.Service  
- Telegram Bot Token: token  
- Telegram Bot Username: username  
  
### TODOs:  
  
- TODO: Implement CreateTables() method to create necessary tables in the database.  
  
### Code Summary:  
  
#### Service Struct:  
  
The `Service` struct represents the core functionality of the Telegram bot. It holds references to the database handler, AI service, and Telegram bot instance. Additionally, it stores the bot's token, username, and context for managing the bot's lifecycle.  
  
#### NewService() Function:  
  
This function initializes a new `Service` instance. It takes the Telegram bot token, database handler, and AI service as input. It sets up a context for handling signals like Ctrl+C and creates a new Telegram bot instance using the provided token and options. The bot is configured with various handlers for different commands and callback queries. Finally, it returns the newly created `Service` instance.  
  
#### Start() Function:  
  
This function starts the Telegram bot by calling the `Start()` method on the bot instance. It also ensures that the context is canceled when the bot is stopped, allowing for graceful shutdown.  
  
#### Handlers:  
  
The code includes several handlers for different commands and callback queries. These handlers are responsible for processing user input and performing the necessary actions, such as clearing the chat, providing endpoints, managing models, and handling other bot-related tasks.  
  
#### TODO:  
  
- Implement the `CreateTables()` method to create the necessary tables in the database. This will ensure that the bot can store and retrieve data as needed.  
  
# internal/telegram/util.go  
## Package: telegram  
  
### Imports:  
  
* context  
* hellper/internal/ai  
* log  
* strconv  
* github.com/go-telegram/bot  
* github.com/go-telegram/bot/models  
  
### External Data, Input Sources:  
  
* ai.Endpoint  
* llmModels (array of strings)  
  
### TODOs:  
  
* None found  
  
### Code Summary:  
  
#### DeleteMessageLog:  
  
This function deletes a message from a chat. It takes the function name, bot instance, context, chat ID, and message ID as input. It uses the bot's DeleteMessage method to delete the message and logs any errors that occur.  
  
#### SendChatActionLog:  
  
This function sends a chat action to a chat. It takes the function name, bot instance, context, chat ID, thread ID, and chat action as input. It uses the bot's SendChatAction method to send the action and logs any errors that occur.  
  
#### SendResponseLog:  
  
This function sends a message to a chat. It takes the function name, bot instance, context, and a pointer to a bot.SendMessageParams struct as input. It uses the bot's SendMessage method to send the message and logs any errors that occur. It returns a pointer to the message ID if successful, otherwise nil.  
  
#### CreateEndpointsMarkup:  
  
This function creates an inline keyboard markup for a list of endpoints. It takes a list of ai.Endpoint structs as input and returns a models.InlineKeyboardMarkup struct. It iterates through the endpoints and creates a button for each one, with the endpoint's name as the text and a callback data string containing the endpoint's ID.  
  
#### CreateModelsMarkup:  
  
This function creates an inline keyboard markup for a list of LLMs. It takes a list of strings as input and returns a models.InlineKeyboardMarkup struct. It iterates through the LLMs and creates a button for each one, with the LLM's name as the text and a callback data string containing the LLM's name.  
  
#### CreateResponseMessageParams:  
  
This function creates a bot.SendMessageParams struct for a response message. It takes the chat ID, thread ID, and a boolean indicating whether the chat is a forum as input. It returns a pointer to a bot.SendMessageParams struct with the appropriate chat ID and thread ID based on the input parameters.  
  
  
  
