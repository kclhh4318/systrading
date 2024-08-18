package backtesting

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"tradingbot/internal/config"
	"tradingbot/internal/models"
	"tradingbot/internal/strategy"
)

func TestBacktestingWithMinuteData(t *testing.T) {
	// 환경 설정 로드
	cfg, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Set the Access Token (this should be set correctly in your config)
	if cfg.Exchange.AccessToken == "" {
		t.Fatalf("Access Token is missing, please set a valid access token.")
	}

	// Print the Access Token for debugging (remove this in production)
	log.Printf("Using Access Token: %s", cfg.Exchange.AccessToken)

	log.Printf("Decoded strategy config: %+v", cfg.Strategy)
	log.Printf("Loaded config: %+v", cfg)

	// 분봉 데이터 요청 URL 설정
	url := "https://openapivts.koreainvestment.com:29443/uapi/domestic-stock/v1/quotations/inquire-time-itemchartprice"

	// 요청 생성
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// 헤더 설정
	req.Header.Set("Authorization", "Bearer "+cfg.Exchange.AccessToken)
	req.Header.Set("appkey", cfg.Exchange.APIKey)
	req.Header.Set("appsecret", cfg.Exchange.APISecret)
	req.Header.Set("tr_id", "FHKST03010200")
	req.Header.Set("custtype", "P")

	// 쿼리 파라미터 설정
	query := req.URL.Query()
	query.Add("FID_COND_MRKT_DIV_CODE", "J")
	query.Add("FID_INPUT_ISCD", cfg.TradingPair)
	query.Add("FID_INPUT_HOUR_1", "090000")
	query.Add("FID_ETC_CLS_CODE", "")
	query.Add("FID_PW_DATA_INCU_YN", "N")
	req.URL.RawQuery = query.Encode()

	// 요청 보내기
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 응답 확인
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("API response status code: %v", resp.StatusCode)
	}

	// 응답 바디 읽기
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	log.Printf("Minute data response body: %s", string(body))

	// JSON 파싱
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal response body: %v", err)
	}

	// 데이터 확인
	output2, ok := result["output2"].([]interface{})
	if !ok || len(output2) == 0 {
		t.Fatalf("Unexpected response format: 'output2' field not found or empty")
	}
	log.Printf("Retrieved %d minute data points", len(output2))

	// 전략 테스트
	strat := strategy.NewMovingAverage(cfg.Strategy)
	totalTrades := 0
	for _, data := range output2 {
		dataMap := data.(map[string]interface{})
		signal := strat.Analyze(&models.MarketData{
			StckPrpr: dataMap["stck_prpr"].(string),
		})
		log.Printf("Signal generated: %v", signal.Type)
		if signal.Type != strategy.HoldSignal {
			totalTrades++
		}
	}

	// 결과 검증
	if totalTrades == 0 {
		t.Errorf("Expected some trades, but got %d", totalTrades)
	}

}
