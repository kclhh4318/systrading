package exchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
	"tradingbot/internal/config"
	"tradingbot/internal/models"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

const (
	maxRetries = 3
	retryDelay = 5 * time.Second
)

type KISExchange struct {
	APIKey          string
	APISecret       string
	BaseURL         string
	AuthToken       string
	AuthTokenExpiry time.Time
	AccountNo       string
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
}

func New(cfg config.ExchangeConfig) (*KISExchange, error) {
	ex := &KISExchange{
		APIKey:    cfg.APIKey,
		APISecret: cfg.APISecret,
		BaseURL:   "https://openapivts.koreainvestment.com:29443",
		AccountNo: cfg.AccountNo,
	}

	if err := ex.refreshAuthToken(); err != nil {
		return nil, fmt.Errorf("failed to get auth token: %v", err)
	}

	return ex, nil
}

func (e *KISExchange) refreshAuthToken() error {
	if time.Now().Before(e.AuthTokenExpiry) {
		return nil
	}

	for retries := 0; retries < maxRetries; retries++ {
		token, expiry, err := e.getAuthToken()
		if err == nil {
			e.AuthToken = token
			e.AuthTokenExpiry = expiry
			return nil
		}

		if strings.Contains(err.Error(), "접근토큰 발급 잠시 후 다시 시도하세요") {
			time.Sleep(1 * time.Minute) // 1분 대기 후 다시 시도
		} else {
			return err
		}
	}

	return fmt.Errorf("failed to refresh auth token after retries")
}

func (e *KISExchange) getAuthToken() (string, time.Time, error) {
	url := fmt.Sprintf("%s/oauth2/tokenP", e.BaseURL)
	data := map[string]string{
		"grant_type": "client_credentials",
		"appkey":     e.APIKey,
		"appsecret":  e.APISecret,
	}

	respBody, err := e.sendRequest("POST", url, data)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to get auth token: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse auth token response: %v", err)
	}

	if errorDescription, exists := result["error_description"].(string); exists {
		return "", time.Time{}, fmt.Errorf("error_description: %s", errorDescription)
	}

	token, ok := result["access_token"].(string)
	if !ok {
		return "", time.Time{}, fmt.Errorf("access token not found in response")
	}

	expiry := time.Now().Add(1 * time.Hour)
	return token, expiry, nil
}

func (e *KISExchange) PlaceOrder(signal *models.Signal) (*models.Order, error) {
	var err error
	var order *models.Order

	for i := 0; i < maxRetries; i++ {
		order, err = e.placeOrderInternal(signal)
		if err == nil {
			return order, nil
		}

		if strings.Contains(err.Error(), "unauthorized request") {
			if refreshErr := e.refreshAuthToken(); refreshErr != nil {
				return nil, fmt.Errorf("failed to refresh auth token: %v", refreshErr)
			}
			continue
		}

		log.WithError(err).Warnf("Failed to place order, retrying in %v...", retryDelay)
		time.Sleep(retryDelay)
	}

	return nil, errors.Wrap(err, "failed to place order after multiple retries")
}

func (e *KISExchange) placeOrderInternal(signal *models.Signal) (*models.Order, error) {
	url := fmt.Sprintf("%s/v1/orders", e.BaseURL)
	orderData := map[string]interface{}{
		"pair":       signal.Pair,
		"amount":     signal.Amount,
		"side":       signal.Type,
		"account_no": e.AccountNo,
	}

	respBody, err := e.sendRequest("POST", url, orderData)
	if err != nil {
		return nil, err
	}

	var order models.Order
	if err := json.Unmarshal(respBody, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %v", err)
	}

	order.Status = "placed"
	return &order, nil
}

func (e *KISExchange) GetMarketDataWithRetry(pair string) (*models.MarketData, error) {
	var marketData *models.MarketData
	var err error

	for i := 0; i < maxRetries; i++ {
		marketData, err = e.GetMarketData(pair)
		if err == nil {
			return marketData, nil
		}

		if strings.Contains(err.Error(), "unauthorized request") {
			if refreshErr := e.refreshAuthToken(); refreshErr != nil {
				return nil, fmt.Errorf("failed to refresh auth token: %v", refreshErr)
			}
			continue
		}

		log.WithError(err).Warnf("Failed to get market data, retrying in %v...", retryDelay)
		time.Sleep(retryDelay)
	}
	return nil, errors.Wrap(err, "failed to get market data after multiple retries")
}

func (e *KISExchange) GetMarketData(stockCode string) (*models.MarketData, error) {
	url := fmt.Sprintf("%s/uapi/domestic-stock/v1/quotations/inquire-price", e.BaseURL)

	req, err := e.newAuthorizedRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("fid_cond_mrkt_div_code", "J")
	q.Add("fid_input_iscd", stockCode)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get market data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get market data, status code: %d", resp.StatusCode)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read market data response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse market data response: %v", err)
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
	return e.GetMarketData("041510")
}

func (e *KISExchange) GetBalance() (string, error) {
	url := fmt.Sprintf("%s/uapi/domestic-stock/v1/trading/inquire-account-balance", e.BaseURL)

	req, err := e.newAuthorizedRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("CANO", e.AccountNo)
	q.Add("ACNT_PRDT_CD", "01")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get balance: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get balance, status code: %d", resp.StatusCode)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read balance response: %v", err)
	}

	var balanceData map[string]interface{}
	if err := json.Unmarshal(respBody, &balanceData); err != nil {
		return "", fmt.Errorf("failed to parse balance response: %v", err)
	}

	if output2, ok := balanceData["output2"].([]interface{}); ok && len(output2) > 0 {
		if dnclAmt, ok := output2[0].(map[string]interface{})["dncl_amt"].(string); ok {
			return dnclAmt, nil
		}
	}

	return "", fmt.Errorf("balance information not found in response")
}

func (e *KISExchange) GetHistoricalData(stockCode string, days int) ([]models.MarketData, error) {
	var historicalData []models.MarketData
	end := time.Now()
	start := end.AddDate(0, 0, -days)

	url := fmt.Sprintf("%s/uapi/domestic-stock/v1/quotations/inquire-daily-price", e.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.WithError(err).Error("Failed to create request for historical data")
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", e.AuthToken))
	req.Header.Set("appkey", e.APIKey)
	req.Header.Set("appsecret", e.APISecret)
	req.Header.Set("tr_id", "FHKST01010400")

	q := req.URL.Query()
	q.Add("FID_COND_MRKT_DIV_CODE", "J")     // 주식 시장 구분 코드
	q.Add("FID_INPUT_ISCD", stockCode)       // 종목 코드
	q.Add("FID_PERIOD_DIV_CODE", "D")        // 일별 데이터
	q.Add("FID_ORG_ADJ_PRC", "1")            // 수정 주가 사용 여부
	q.Add("ST_DT", start.Format("20060102")) // 시작일 (YYYYMMDD 형식)
	q.Add("EN_DT", end.Format("20060102"))   // 종료일 (YYYYMMDD 형식)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("Failed to get historical data from API")
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read response body")
		return nil, err
	}

	// 응답 본문 디버깅
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.WithError(err).Error("Failed to unmarshal response body")
		return nil, err
	}

	output, ok := result["output"].([]interface{})
	if !ok {
		log.Error("Unexpected response format: 'output' field not found")
		return nil, fmt.Errorf("unexpected response format")
	}

	for _, item := range output {
		data, ok := item.(map[string]interface{})
		if !ok {
			log.Warn("Unexpected item format in output")
			continue
		}

		marketData := models.MarketData{
			StckPrpr: data["stck_clpr"].(string), // 종가 사용
		}

		historicalData = append(historicalData, marketData)
		log.Infof("Parsed market data: %+v", marketData)
	}

	log.Infof("Total %d data points retrieved for stock code %s", len(historicalData), stockCode)

	return historicalData, nil
}

func (e *KISExchange) GetMinuteData(stockCode string) ([]models.MarketData, error) {
	url := fmt.Sprintf("%s/uapi/domestic-stock/v1/quotations/inquire-time-itemchartprice", e.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.WithError(err).Error("Failed to create request for minute data")
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", e.AuthToken))
	req.Header.Set("appkey", e.APIKey)
	req.Header.Set("appsecret", e.APISecret)
	req.Header.Set("tr_id", "FHKST01010400") // API 엔드포인트에 따라 이 값이 달라질 수 있습니다.

	q := req.URL.Query()
	q.Add("FID_COND_MRKT_DIV_CODE", "J")
	q.Add("FID_INPUT_ISCD", stockCode)
	q.Add("FID_PERIOD_DIV_CODE", "M1") // 1분봉 데이터 요청
	req.URL.RawQuery = q.Encode()

	// 요청한 URL과 헤더를 로그로 출력
	log.Infof("Requesting minute data with URL: %s", req.URL.String())
	log.Infof("Request headers: Authorization: %s, AppKey: %s, AppSecret: %s", e.AuthToken, e.APIKey, e.APISecret)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("Failed to get minute data from API")
		return nil, err
	}
	defer resp.Body.Close()

	log.Infof("API response status code: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get minute data, status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read response body")
		return nil, err
	}

	log.Infof("Minute data response body: %s", string(body))

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.WithError(err).Error("Failed to unmarshal response body")
		return nil, err
	}

	output, ok := result["output"].([]interface{})
	if !ok {
		log.Error("Unexpected response format: 'output' field not found")
		return nil, fmt.Errorf("unexpected response format")
	}

	if resp.StatusCode != http.StatusOK {
		log.Errorf("Failed to get minute data, status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("failed to get minute data, status code: %d", resp.StatusCode)
	}

	var minuteData []models.MarketData
	for _, item := range output {
		data, ok := item.(map[string]interface{})
		if !ok {
			log.Warn("Unexpected item format in output")
			continue
		}

		minuteData = append(minuteData, models.MarketData{
			StckPrpr: data["stck_clpr"].(string), // 종가 사용
		})
	}

	log.Infof("Total %d data points retrieved for stock code %s", len(minuteData), stockCode)

	return minuteData, nil
}

func (e *KISExchange) sendRequest(method, url string, data interface{}) ([]byte, error) {
	var reqBody []byte
	var err error

	if data != nil {
		reqBody, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request data: %v", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.AuthToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (e *KISExchange) newAuthorizedRequest(method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.AuthToken))
	req.Header.Set("appKey", e.APIKey)
	req.Header.Set("appSecret", e.APISecret)

	return req, nil
}

func GetAccessToken(appKey, appSecret string) (string, error) {
	url := "https://openapi.koreainvestment.com:9443/oauth2/tokenP"

	data := map[string]string{
		"grant_type": "client_credentials",
		"appkey":     appKey,
		"appsecret":  appSecret,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, body)
	}

	var authResponse AuthResponse
	if err := json.Unmarshal(body, &authResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return authResponse.AccessToken, nil
}
