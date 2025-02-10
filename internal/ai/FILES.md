# internal/ai/database.go  
```  
  
  
# internal/ai/inference.go  
## Package: ai  
  
### Imports:  
- context  
- errors  
- github.com/tmc/langchaingo/llms  
  
### External Data, Input Sources:  
- userId: int64  
- chatId: int64  
- threadId: int64  
- prompt: string  
  
### TODOs:  
- Implement Langgraph implementation from pkg or separate go lib for generating content.  
  
### Summary:  
The code defines a function called `Inference` that takes userId, chatId, threadId, and prompt as input. It first retrieves the session and handler for the given userId. If the session is missing or the handler is not found, it retrieves a token and updates the handler accordingly.  
  
The function then updates the history and retrieves the history for the given parameters. It calls the handler's `GenerateContent` method to generate a response using the history. If the response is empty, it returns an error.  
  
Finally, the function updates the history and usage for the given parameters and returns the generated text response.  
  
# internal/ai/models.go  
## Package: ai  
  
### Imports:  
- encoding/json  
- net/http  
- net/url  
  
### External Data, Input Sources:  
- OpenAI API endpoint  
- OpenAI API token  
  
### TODOs:  
- None  
  
### Summary:  
#### GetModelsList Function:  
This function retrieves a list of available OpenAI models from the OpenAI API. It takes the API endpoint and token as input parameters. The function constructs a GET request to the specified endpoint, sets the authorization header with the provided token, and sends the request using the default HTTP client. The response is then decoded into an OpenAIModelsResponse object, and the IDs of the available models are extracted and returned as a slice of strings.  
  
# internal/ai/service.go  
## Package: ai  
  
### Imports:  
- errors  
- hellper/internal/database  
- sync  
- github.com/tmc/langchaingo/llms/openai  
  
### External Data, Input Sources:  
- database.Handler (from hellper/internal/database)  
  
### TODOs:  
- None found  
  
### Summary:  
#### Service:  
The `Service` struct manages the LLM handlers for different users. It has two fields:  
- `LLMHandlers`: A sync.Map that stores the LLM handlers for each user.  
- `DBHandler`: A database handler for interacting with the database.  
  
#### NewAIService:  
The `NewAIService` function creates a new instance of the `Service` struct and initializes it with the provided database handler. It also calls the `CreateTables` method to create the necessary tables in the database.  
  
#### GetHandler:  
The `GetHandler` method retrieves the LLM handler for a given user ID. It first checks if the handler exists in the `LLMHandlers` map. If it does, it casts the handler to an `openai.LLM` and returns it. If the handler is not found or cannot be cast, it returns an error.  
  
#### DropHandler:  
The `DropHandler` method removes the LLM handler for a given user ID from the `LLMHandlers` map.  
  
#### UpdateHandler:  
The `UpdateHandler` method updates the LLM handler for a given user ID. It creates a new `openai.LLM` instance with the provided token, model, and endpoint URL. It then stores the new handler in the `LLMHandlers` map and returns the updated handler.  
  
  
  
