package langchain

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
)

type OpenAIDataObject struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

type OpenAIModelsResponse struct {
	Data []OpenAIDataObject `json:"data"`
}

func GetModelsList(api_token, ai_endpoint string) []string {
	urlPath, err := url.JoinPath(ai_endpoint, "v1", "models")
	if err != nil {
		log.Printf("GetModelsList: error joining path: %v\n", err)
		return []string{}
	}
	req, err := http.NewRequest("GET", urlPath, nil)
	req.Header.Set("Authorization", "Bearer "+api_token)
	if err != nil {
		log.Printf("GetModelsList: error creating request: %v\n", err)
		return []string{}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("GetModelsList: error doing http request: %v\n", err)
		return []string{}
	}

	modelsResp := OpenAIModelsResponse{}
	err = json.NewDecoder(resp.Body).Decode(&modelsResp)
	if err != nil {
		log.Printf("GetModelsList: error parsing response body: %v\n", err)
		return []string{}
	}

	modelsList := []string{}
	for _, obj := range modelsResp.Data {
		modelsList = append(modelsList, obj.ID)
	}

	return modelsList
}
