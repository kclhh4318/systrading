package exchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"tradingbot/internal/config"
	"tradingbot/internal/models"
)

type KISExchange struct {
	APIKey    string
	APISecret string
	BaseURL   string
	AuthToken string
	AccountNo string
}

func New(cfg config.ExchangeConfig) (*KISExchange, error) {
	ex := &KISExchange{
		APIKey:    cfg.APIKey,
		APISecret: cfg.APISecret,
		BaseURL:   "https://openapi.koreainvestment.com:9443", // 실전 투자용 엔드포인트
		AccountNo: cfg.AccountNo,
	}

	fmt.Printf("API Key: %s\n", ex.APIKey)
	fmt.Printf("API Secret: %s\n", ex.APISecret)
	fmt.Printf("Account No: %s\n", ex.AccountNo)

	token, err := ex.getAuthToken()
	if err != nil {
		return nil, err
	}
	ex.AuthToken = token

	return ex, nil
}

func (e *KISExchange) getAuthToken() (string, error) {
	url := fmt.Sprintf("%s/oauth2/tokenP", e.BaseURL)
	data := fmt.Sprintf(`{
		"grant_type": "client_credentials",
		"appkey": "%s",
		"appsecret": "%s"
	}`, e.APIKey, e.APISecret)

	req, err := http.NewRequest("POST", url, strings.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 응답 내용을 출력하여 확인
	fmt.Println("Response Body:", string(respBody))

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	// 에러 메시지 확인
	if errorDescription, exists := result["error_description"].(string); exists {
		return "", fmt.Errorf("error_description: %s", errorDescription)
	}

	token, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("access token not found in response")
	}

	return token, nil
}

func (e *KISExchange) PlaceOrder(signal *models.Signal) (*models.Order, error) {
	url := fmt.Sprintf("%s/v1/orders", e.BaseURL)
	orderData := map[string]interface{}{
		"pair":       signal.Pair,
		"amount":     signal.Amount,
		"side":       signal.Type,
		"account_no": e.AccountNo,
	}

	body, err := json.Marshal(orderData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.AuthToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to place order, status code: %d", resp.StatusCode)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var order models.Order
	if err := json.Unmarshal(respBody, &order); err != nil {
		return nil, err
	}

	return &order, nil
}

func (e *KISExchange) GetMarketData(pair string) (*models.MarketData, error) {
	url := fmt.Sprintf("%s/uapi/domestic-stock/v1/quotations/inquire-price", e.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.AuthToken))
	req.Header.Set("appKey", e.APIKey)
	req.Header.Set("appSecret", e.APISecret)
	req.Header.Set("tr_id", "FHKST01010100")

	q := req.URL.Query()
	q.Add("fid_cond_mrkt_div_code", "J")
	q.Add("fid_input_iscd", pair)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get market data, status code: %d", resp.StatusCode)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println("Market Data Response Body:", string(respBody))

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	var marketData models.MarketData
	if data, ok := result["output"].(map[string]interface{}); ok {
		marketData.StckPrpr = data["stck_prpr"].(string)
	} else {
		return nil, fmt.Errorf("market data not found in response")
	}

	return &marketData, nil
}

func (e *KISExchange) GetSamsungPrice() (*models.MarketData, error) {
	return e.GetMarketData("005930")
}

func (e *KISExchange) GetBalance() (interface{}, error) {
	url := fmt.Sprintf("%s/uapi/domestic-stock/v1/trading/inquire-account-balance", e.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.AuthToken))
	req.Header.Set("appKey", e.APIKey)
	req.Header.Set("appSecret", e.APISecret)
	req.Header.Set("tr_id", "CTRP6548R")
	req.Header.Set("custtype", "P")

	q := req.URL.Query()
	q.Add("CANO", e.AccountNo)
	q.Add("ACNT_PRDT_CD", "01")
	q.Add("INQR_DVSN_1", "")
	q.Add("BSPR_BF_DT_APLY_YN", "")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get balance, status code: %d", resp.StatusCode)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println("Balance Response Body:", string(respBody))

	var balanceData map[string]interface{}
	if err := json.Unmarshal(respBody, &balanceData); err != nil {
		return nil, err
	}

	// Log detailed balance information
	output1, ok := balanceData["output1"].([]interface{})
	if ok {
		for i, item := range output1 {
			fmt.Printf("Output1 Item %d: %+v\n", i, item)
		}
	}
	output2, ok := balanceData["output2"].(map[string]interface{})
	if ok {
		fmt.Printf("Output2: %+v\n", output2)
	}

	return balanceData, nil
}
