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

func New(cfg config.ExchangeConfig) (*KISExchange, error) {
	ex := &KISExchange{
		APIKey:    cfg.APIKey,
		APISecret: cfg.APISecret,
		BaseURL:   "https://openapivts.koreainvestment.com:29443",
		AccountNo: cfg.AccountNo,
	}

	err := ex.refreshAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %v", err)
	}

	return ex, nil
}

func (e *KISExchange) refreshAuthToken() error {
	if time.Now().Before(e.AuthTokenExpiry) {
		// 토큰이 아직 유효한 경우, 새로 발급받지 않음
		return nil
	}

	token, expiry, err := e.getAuthToken()
	if err != nil {
		return err
	}
	e.AuthToken = token
	e.AuthTokenExpiry = expiry
	return nil
}

func (e *KISExchange) getAuthToken() (string, time.Time, error) {
	url := fmt.Sprintf("%s/oauth2/tokenP", e.BaseURL)
	data := fmt.Sprintf(`{
        "grant_type": "client_credentials",
        "appkey": "%s",
        "appsecret": "%s"
    }`, e.APIKey, e.APISecret)

	req, err := http.NewRequest("POST", url, strings.NewReader(data))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", time.Time{}, err
	}

	if errorDescription, exists := result["error_description"].(string); exists {
		return "", time.Time{}, fmt.Errorf("error_description: %s", errorDescription)
	}

	token, ok := result["access_token"].(string)
	if !ok {
		return "", time.Time{}, fmt.Errorf("access token not found in response")
	}

	// 토큰 만료 시간 계산 (예시로 1시간 후 만료로 설정)
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
			// 토큰 갱신을 시도
			err = e.refreshAuthToken()
			if err != nil {
				log.WithError(err).Error("Failed to refresh auth token")
				return nil, err
			}
			// 토큰 갱신 후 재시도
			continue
		}

		log.WithError(err).Warnf("Failed to place order, retrying in %v...", retryDelay)
		time.Sleep(retryDelay)
	}

	// 모든 재시도 후에도 실패한 경우
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

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized request, token might be expired")
	}

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

	return &models.Order{
		Pair:   signal.Pair,
		Type:   signal.Type,
		Amount: signal.Amount,
		Status: "placed",
	}, nil
}

func (e *KISExchange) GetMarketDataWithRetry(pair string) (*models.MarketData, error) {
	var marketData *models.MarketData
	var err error

	for i := 0; i < maxRetries; i++ {
		marketData, err = e.GetMarketData(pair)
		if err == nil {
			return marketData, nil
		}

		// Unauthorized error handling (e.g., 401 status)
		if strings.Contains(err.Error(), "unauthorized request") {
			if refreshErr := e.refreshAuthToken(); refreshErr != nil {
				log.WithError(refreshErr).Error("Failed to refresh auth token")
				return nil, refreshErr
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

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.AuthToken))
	req.Header.Set("appKey", e.APIKey)
	req.Header.Set("appSecret", e.APISecret)
	req.Header.Set("tr_id", "FHKST01010100") // 트랜잭션 ID 설정

	q := req.URL.Query()
	q.Add("fid_cond_mrkt_div_code", "J") // 시장 구분 코드 (J for 주식)
	q.Add("fid_input_iscd", stockCode)   // 종목 코드 (예: "005930" for 삼성전자)
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
	return e.GetMarketData("041510")
}

func (e *KISExchange) GetBalance() (string, error) {
	url := fmt.Sprintf("%s/uapi/domestic-stock/v1/trading/inquire-account-balance", e.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
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
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get balance, status code: %d", resp.StatusCode)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var balanceData map[string]interface{}
	if err := json.Unmarshal(respBody, &balanceData); err != nil {
		return "", err
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
	q.Add("FID_COND_MRKT_DIV_CODE", "J")
	q.Add("FID_INPUT_ISCD", stockCode)
	q.Add("FID_PERIOD_DIV_CODE", "M1") // 일별 데이터
	q.Add("FID_ORG_ADJ_PRC", "1")
	q.Add("ST_DT", start.Format("20060102"))
	q.Add("EN_DT", end.Format("20060102"))
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

	log.Infof("Historical data response body: %s", string(body))

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

func (e *KISExchange) GetMinuteData(stockCode string, minutes int) ([]models.MarketData, error) {
	var minuteData []models.MarketData

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
	req.Header.Set("tr_id", "FHKST01010400") // 엔드포인트 설정

	q := req.URL.Query()
	q.Add("FID_COND_MRKT_DIV_CODE", "J")
	q.Add("FID_INPUT_ISCD", stockCode)
	q.Add("FID_PERIOD_DIV_CODE", "M1") // 1분봉 데이터 요청
	q.Add("FID_ORG_ADJ_PRC", "1")
	q.Add("FID_INPUT_HOUR_1", "0900") // 시작 시간
	q.Add("FID_INPUT_HOUR_2", "1500") // 종료 시간
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("Failed to get minute data from API")
		return nil, err
	}
	defer resp.Body.Close()

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
