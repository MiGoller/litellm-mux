package models

type ModelInfoResponse struct {
	Data []ModelData `json:"data"`
}

type ModelData struct {
	ModelName    string                 `json:"model_name"`
	ModelInfo    map[string]interface{} `json:"model_info"`
	LitellmParams map[string]interface{} `json:"litellm_params"`
}
