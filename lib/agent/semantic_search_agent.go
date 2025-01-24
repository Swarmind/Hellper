package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"

	//"github.com/tmc/langgraphgo/graph"
	"github.com/JackBekket/hellper/lib/embeddings"
	"github.com/JackBekket/langgraphgo/graph"
)

/** My current vision of this mechanism is a graph. So each agent can be represented as graph. Each node is usually single action in <turn_of_dialog>. Graphs is connected with themselves through edges, which represent
  relations whithin graphs. Each graph can be conditional or direct. If we need to reorder graph we can simply alter entry_point instead of rewriting code of dialog itseelf every time.
  Each graph can also be represented graphically.


    This is OneShot agent example
    It does not have memory by itself, but memory (history of previouse messages) can be passed as optional parameter
*/






func OneShotRun(prompt string, history_state ...llms.MessageContent) string{

  model_name := "tiger-gemma-9b-v1-i1"    // should be settable?
  _ = godotenv.Load()
          ai_url := os.Getenv("AI_ENDPOINT")          //TODO: should be global?
          api_token := os.Getenv("ADNIN_KEY")
          //db_link := os.Getenv("EMBEDDINGS_DB_URL")

  model, err := openai.New(
    openai.WithToken(api_token),
    //openai.WithBaseURL("http://localhost:8080"),
    openai.WithBaseURL(ai_url),
    openai.WithModel(model_name),
    openai.WithAPIVersion("v1"),
  )
  if err != nil {
    log.Fatal(err)
  }

  //completion_test := model.GenerateContent()

  
  // Operation with message STATE stack

  agentState := []llms.MessageContent{
    llms.TextParts(llms.ChatMessageTypeSystem, "You are an agent that has access to a semanticSearch tool. Please use this tool to get user information they are looking for."),
  }
  intialState := []llms.MessageContent{
    llms.TextParts(llms.ChatMessageTypeSystem, "Below a current conversation between user and helpful AI assistant. Your task will be in the next system message"),
  }

  if len(history_state) > 0 {                   // if there are previouse message state then we first load it into message state
    // Access the first element of the slice
    history := history_state
    // ... use the history variable as needed
    for _, message := range history {
      intialState = append(intialState, message)  // load history as initial state
    }
    intialState = append(
      intialState,   
      agentState...,        // append agent system prompt
    )
    intialState = append(
      intialState,  
      llms.TextParts(llms.ChatMessageTypeHuman, prompt),  //append user input (!)
    )
  } else {
    intialState = agentState    //history is empty -- load agentState as initial_state and append user prompt
    intialState = append(
      intialState,  
      llms.TextParts(llms.ChatMessageTypeHuman, prompt),
    )
    
    
  }


// toolS definition interfaces
  tools := []llms.Tool{
    {
      Type: "function",
      Function: &llms.FunctionDefinition{
        Name:        "search",
        Description: "Preforms Duck Duck Go web search",
        Parameters: map[string]any{
          "type": "object",
          "properties": map[string]any{
            "query": map[string]any{
              "type":        "string",
              "description": "The search query",
            },
          },
        },
      },
    },
    {
      Type: "function",
      Function: &llms.FunctionDefinition{
        Name:        "semanticSearch",
        Description: "Performs semantic search using a vector store",
        Parameters: map[string]any{
          "type": "object",
          "properties": map[string]any{
            "query": map[string]any{
              "type":        "string",
              "description": "The search query",
            },
            "collection": map[string]any{                     //TODO: there should NOT exist arguments which called NAME cause it cause COLLISION with actual function name.    .....more like confusion then collision so there are no error
              "type":        "string",
              "description": "name of collection store in which we perform the search",
            },
          }, 
        },
      },
    },
  }




// AGENT NODE
/** We are telling agent, that it should response withTools, giving it function signatures defined earlier. 
    if agent get response from conditional edge like 'yes, use x function with this signatures and this json object as input parameters -- it will match with predefined pointer to semanticSearch function and it will make a toolCall
    then it will append toolCall to message state.
    Note, that agent can call few toolCalls and all of them can be append here. toolCalls may be done parallel (I guess) */
  agent := func(ctx context.Context, state []llms.MessageContent) ([]llms.MessageContent, error) {
   

    lastMsg := state[len(state)-1]
        if lastMsg.Role == "tool" {   // If we catch response from tool then we use this response
          response, err := model.GenerateContent(ctx, state)
          if err != nil {
            return state, err
          }
          msg := llms.TextParts(llms.ChatMessageTypeAI, response.Choices[0].Content)
          state = append(state, msg)
          return state,nil


  }    else {   // If it is first interaction then we call tools
    response, err := model.GenerateContent(ctx, state, llms.WithTools(tools))   // AI call tool function.. in this step it just put call in messages stack
    if err != nil {
      return state, err
    }
    msg := llms.TextParts(llms.ChatMessageTypeAI, response.Choices[0].Content)

    if len(response.Choices[0].ToolCalls) > 0 {
      for _, toolCall := range response.Choices[0].ToolCalls {
        if toolCall.FunctionCall.Name == "semanticSearch" {       // AI catch that there is a function call in messages, so *now* it actually calls the function.

          msg.Parts = append(msg.Parts, toolCall) // Add result to messages stack

        }
      }
    }
    state = append(state, msg)  
    return state, nil
  }
}


// TOOL FUNCTIONS

  // Custom semantic search function for working with vector-store information
  semanticSearch := func(ctx context.Context, state []llms.MessageContent) ([]llms.MessageContent, error) {
    lastMsg := state[len(state)-1]

    for _, part := range lastMsg.Parts {
      toolCall, ok := part.(llms.ToolCall)
      if ok && toolCall.FunctionCall.Name == "semanticSearch" {

        // TODO: Extract query and store parameters from the arguments
        // ... (logic to extract necessary values for SemanticSearch call)
        var args struct {
          Query string `json:"query"`
          //Store string `json:"store"`
          //Options []map[string]any `json:"options"`
          Collection string `json:"collection"`   //TODO: ALWAYS CHECK THIS JSON REFERENCE WHEN ALTERING VARS
        }
        if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
          // Handle any errors in deserializing the arguments
    log.Println("error unmurshal json")
          return state, err
        }
        // Extract query from the args structure
        searchQuery := args.Query

        //get env
        _ = godotenv.Load()
        ai_url := os.Getenv("AI_ENDPOINT")          //TODO: should be global?   .. there are global, there might be resetting.
        api_token := os.Getenv("ADNIN_KEY")
        db_link := os.Getenv("EMBEDDINGS_DB_URL")

        log.Println("Collection Name: ", args.Collection)
        log.Println("db_link: ", db_link)


        // Retrieve your vector store based on the store value in the args
        // You'll likely need to have a method for getting the vector store based
        // on the store string ("store" value in the args)
        store, err := embeddings.GetVectorStoreWithOptions(ai_url,api_token,db_link,args.Collection) // TODO: changed argument 'Name' to 'CollectionName' or something like that
        if err != nil {
          // Handle errors in retrieving the vector store
    log.Println("error getting store")
          return state, err
        }

        log.Println("store:", store)

        maxResults := 1 // Set your desired maxResults here
        //options := args.Options // Pass in any additional options as needed

        // Call your SemanticSearch function here
        searchResults, err := embeddings.SemanticSearch(
          searchQuery, 
          maxResults,
          store, // Pass in your vector store
         // options, // Pass in any additional options you need
        )

        if err != nil {
            log.Printf("semantic search error: %v", err)
            return state, err
        }

        
        // Format and return search results
        // ... (process and format search results from SemanticSearch)
        //toolResponse := []string{} // Initialize an empty slice to store extracted text
        toolResponse := ""
        for _, result := range searchResults {
          //toolResponse = append(toolResponse, result.PageContent) 
          toolResponse += result.PageContent + "\n"
          
        }

        msg := llms.MessageContent{
          Role: llms.ChatMessageTypeTool,
          Parts: []llms.ContentPart{
            llms.ToolCallResponse{
              ToolCallID: toolCall.ID,
              Name:       toolCall.FunctionCall.Name,
              Content:    toolResponse,
            },
          },
        }
        state = append(state, msg)
      }
    }
    return state, nil
}



 //CONDITIONS funcs

  // condition function, which defines whether or not to use semanticSearch tool. we have access to semanticSearch itself in main thread through a pointer to this function. So if llm says 'yes, use this function with x signatures` -- it will match to a pointer and x function will be called.`
  shouldSearchDocuments := func(ctx context.Context, state []llms.MessageContent) string {
  
    // this function (I suppose) can be reworked to work with a *set* of a functions, not just one func.

    lastMsg := state[len(state)-1]
    for _, part := range lastMsg.Parts {
      toolCall, ok := part.(llms.ToolCall)

      if ok && toolCall.FunctionCall.Name == "semanticSearch"  {
        log.Printf("agent should use SemanticSearch (embeddings similarity search aka DocumentsSearch)")
        return "semanticSearch"
      }
    }

    return graph.END  // never reach this point, should be removed?
  }




// MAIN WORKFLOW
  workflow := graph.NewMessageGraph()

  workflow.AddNode("agent", agent)
  //workflow.AddNode("search", search)
  workflow.AddNode("semanticSearch", semanticSearch)

  workflow.SetEntryPoint("agent")
  workflow.AddConditionalEdge("agent", shouldSearchDocuments)
  workflow.AddEdge("semanticSearch", "agent")

  app, err := workflow.Compile()
  if err != nil {
    log.Printf("error: %v", err)
    return fmt.Sprintf("error :%v", err)
  }

  response, err := app.Invoke(context.Background(), intialState)
  if err != nil {
    log.Printf("error: %v", err)
    return fmt.Sprintf("error :%v", err)
  }

  lastMsg := response[len(response)-1]
  log.Printf("last msg: %v", lastMsg.Parts[0]) 
  result := lastMsg.Parts[0]
  result_str := fmt.Sprintf("%v", result)
  return result_str
}
