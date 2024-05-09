package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// var (
// 	apiName     = flag.String("api_name", "", "API name to be used")
// 	filePath    = flag.String("file_path", "", "File path to be used")
// 	groupID     = flag.String("group_id", "", "Group ID to be used")
// 	featureID   = flag.String("feature_id", "", "Feature ID to be used")
// 	groupName   = flag.String("group_name", "", "Group name to be used")
// 	groupInfo   = flag.String("group_info", "", "Group info to be used")
// 	featureInfo = flag.String("feature_info", "", "Feature info to be used")
// 	topK        = flag.Int("top_k", 0, "Search result top k to be used")
// )

type GenReqURL struct {
	host string
	path string
}

func (g *GenReqURL) sha256base64(data []byte) string {
	hash := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}

func (g *GenReqURL) parseURL(requestURL string) error {
	stidx := strings.Index(requestURL, "://")
	if stidx == -1 {
		return errors.New("invalid request url: " + requestURL)
	}
	host := requestURL[stidx+3:]
	edidx := strings.Index(host, "/")
	if edidx <= 0 {
		return errors.New("invalid request url: " + requestURL)
	}
	g.path = host[edidx:]
	g.host = host[:edidx]
	return nil
}

func (g *GenReqURL) assembleWSAuthURL(requestURL, apiKey, apiSecret, method string) (string, error) {
	err := g.parseURL(requestURL)
	if err != nil {
		return "", err
	}

	loc, _ := time.LoadLocation("GMT")
	now := time.Now().In(loc)
	date := now.Format(time.RFC1123)
	signatureOrigin := fmt.Sprintf("host: %s\ndate: %s\n%s %s HTTP/1.1", g.host, date, method, g.path)

	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(signatureOrigin))
	signatureSHA := base64.StdEncoding.EncodeToString(h.Sum(nil))

	authorizationOrigin := fmt.Sprintf(`api_key="%s", algorithm="%s", headers="%s", signature="%s"`,
		apiKey, "hmac-sha256", "host date request-line", signatureSHA)
	authorization := base64.StdEncoding.EncodeToString([]byte(authorizationOrigin))

	values := url.Values{
		"host":          []string{g.host},
		"date":          []string{date},
		"authorization": []string{authorization},
	}

	return requestURL + "?" + values.Encode(), nil
}

func genCreateFeatureReqBody(appId, filePath, groupId, featureId, featureInfo string) ([]byte, error) {
	audioBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{
		"header": map[string]interface{}{
			"app_id": appId,
			"status": 3,
		},
		"parameter": map[string]interface{}{
			"s782b4996": map[string]interface{}{
				"groupId":     groupId,
				"featureId":   featureId,
				"featureInfo": featureInfo,
				"createFeatureRes": map[string]interface{}{
					"encoding": "utf8",
					"compress": "raw",
					"format":   "json",
				},
			},
		},
		"payload": map[string]interface{}{
			"resource": map[string]interface{}{
				"encoding":    "lame",
				"sample_rate": 16000,
				"channels":    1,
				"bit_depth":   16,
				"status":      3,
				"audio":       base64.StdEncoding.EncodeToString(audioBytes),
			},
		},
	}
	return json.Marshal(body)
}

func genCreateGroupReqBody(appId, groupId, groupName, groupInfo string) ([]byte, error) {
	body := map[string]interface{}{
		"header": map[string]interface{}{
			"app_id": appId,
			"status": 3,
		},
		"parameter": map[string]interface{}{
			"s782b4996": map[string]interface{}{
				"func":      "createGroup",
				"groupId":   groupId,
				"groupName": groupName,
				"groupInfo": groupInfo,
				"createGroupRes": map[string]interface{}{
					"encoding": "utf8",
					"compress": "raw",
					"format":   "json",
				},
			},
		},
	}
	return json.Marshal(body)
}

func genDeleteFeatureReqBody(appId, groupId, featureId string) ([]byte, error) {
	body := map[string]interface{}{
		"header": map[string]interface{}{
			"app_id": appId,
			"status": 3,
		},
		"parameter": map[string]interface{}{
			"s782b4996": map[string]interface{}{
				"func":      "deleteFeature",
				"groupId":   groupId,
				"featureId": featureId,
				"deleteFeatureRes": map[string]interface{}{
					"encoding": "utf8",
					"compress": "raw",
					"format":   "json",
				},
			},
		},
	}
	return json.Marshal(body)
}

func genQueryFeatureListReqBody(appId, groupId string) ([]byte, error) {
	body := map[string]interface{}{
		"header": map[string]interface{}{
			"app_id": appId,
			"status": 3,
		},
		"parameter": map[string]interface{}{
			"s782b4996": map[string]interface{}{
				"func":    "queryFeatureList",
				"groupId": groupId,
				"queryFeatureListRes": map[string]interface{}{
					"encoding": "utf8",
					"compress": "raw",
					"format":   "json",
				},
			},
		},
	}
	return json.Marshal(body)
}

func genSearchFeaReqBody(appId, filePath, groupId string, topK int) ([]byte, error) {
	audioBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{
		"header": map[string]interface{}{
			"app_id": appId,
			"status": 3,
		},
		"parameter": map[string]interface{}{
			"s782b4996": map[string]interface{}{
				"func":    "searchFea",
				"groupId": groupId,
				"topK":    topK,
				"searchFeaRes": map[string]interface{}{
					"encoding": "utf8",
					"compress": "raw",
					"format":   "json",
				},
			},
		},
		"payload": map[string]interface{}{
			"resource": map[string]interface{}{
				"encoding":    "lame",
				"sample_rate": 16000,
				"channels":    1,
				"bit_depth":   16,
				"status":      3,
				"audio":       base64.StdEncoding.EncodeToString(audioBytes),
			},
		},
	}
	return json.Marshal(body)
}

func genSearchScoreFeaReqBody(appId, filePath, groupId, featureId string) ([]byte, error) {
	audioBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{
		"header": map[string]interface{}{
			"app_id": appId,
			"status": 3,
		},
		"parameter": map[string]interface{}{
			"s782b4996": map[string]interface{}{
				"func":      "searchScoreFea",
				"groupId":   groupId,
				"featureId": featureId,
				"searchScoreFeaRes": map[string]interface{}{
					"encoding": "utf8",
					"compress": "raw",
					"format":   "json",
				},
			},
		},
		"payload": map[string]interface{}{
			"resource": map[string]interface{}{
				"encoding":    "lame",
				"sample_rate": 16000,
				"channels":    1,
				"bit_depth":   16,
				"status":      3,
				"audio":       base64.StdEncoding.EncodeToString(audioBytes),
			},
		},
	}
	return json.Marshal(body)
}

func genUpdateFeatureReqBody(appId, filePath, groupId, featureId, featureInfo string) ([]byte, error) {
	audioBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{
		"header": map[string]interface{}{
			"app_id": appId,
			"status": 3,
		},
		"parameter": map[string]interface{}{
			"s782b4996": map[string]interface{}{
				"func":        "updateFeature",
				"groupId":     groupId,
				"featureId":   featureId,
				"featureInfo": featureInfo,
				"updateFeatureRes": map[string]interface{}{
					"encoding": "utf8",
					"compress": "raw",
					"format":   "json",
				},
			},
		},
		"payload": map[string]interface{}{
			"resource": map[string]interface{}{
				"encoding":    "lame",
				"sample_rate": 16000,
				"channels":    1,
				"bit_depth":   16,
				"status":      3,
				"audio":       base64.StdEncoding.EncodeToString(audioBytes),
			},
		},
	}
	return json.Marshal(body)
}

func genDeleteGroupReqBody(appId, groupId string) ([]byte, error) {
	body := map[string]interface{}{
		"header": map[string]interface{}{
			"app_id": appId,
			"status": 3,
		},
		"parameter": map[string]interface{}{
			"s782b4996": map[string]interface{}{
				"func":    "deleteGroup",
				"groupId": groupId,
				"deleteGroupRes": map[string]interface{}{
					"encoding": "utf8",
					"compress": "raw",
					"format":   "json",
				},
			},
		},
	}
	return json.Marshal(body)
}

type reqInfo struct {
	apiKey      string
	apiSecret   string
	appId       string
	apiName     string
	audio       string
	groupId     string
	featureId   string
	groupName   string
	groupInfo   string
	featureInfo string
	topK        int
}

func genReqBody(r *reqInfo) ([]byte, error) {
	apiName := r.apiName
	appID := r.appId
	audio := r.audio
	groupID := r.groupId
	featureID := r.featureId
	groupName := r.groupName
	groupInfo := r.groupInfo
	featureInfo := r.featureInfo
	topK := r.topK

	if apiName == "createFeature" {
		body := map[string]interface{}{
			"header": map[string]interface{}{
				"app_id": appID,
				"status": 3,
			},
			"parameter": map[string]interface{}{
				"s782b4996": map[string]interface{}{
					"func":        "createFeature",
					"groupId":     groupID,
					"featureId":   featureID,
					"featureInfo": featureInfo,
					"createFeatureRes": map[string]interface{}{
						"encoding": "utf8",
						"compress": "raw",
						"format":   "json",
					},
				},
			},
			"payload": map[string]interface{}{
				"resource": map[string]interface{}{
					"encoding":    "lame",
					"sample_rate": 16000,
					"channels":    1,
					"bit_depth":   16,
					"status":      3,
					"audio":       audio,
				},
			},
		}
		return json.Marshal(body)
	} else if apiName == "createGroup" {
		body := map[string]interface{}{
			"header": map[string]interface{}{
				"app_id": appID,
				"status": 3,
			},
			"parameter": map[string]interface{}{
				"s782b4996": map[string]interface{}{
					"func":      "createGroup",
					"groupId":   groupID,
					"groupName": groupName,
					"groupInfo": groupInfo,
					"createGroupRes": map[string]interface{}{
						"encoding": "utf8",
						"compress": "raw",
						"format":   "json",
					},
				},
			},
		}
		return json.Marshal(body)
	} else if apiName == "deleteFeature" {
		body := map[string]interface{}{
			"header": map[string]interface{}{
				"app_id": appID,
				"status": 3,
			},
			"parameter": map[string]interface{}{
				"s782b4996": map[string]interface{}{
					"func":      "deleteFeature",
					"groupId":   groupID,
					"featureId": featureID,
					"deleteFeatureRes": map[string]interface{}{
						"encoding": "utf8",
						"compress": "raw",
						"format":   "json",
					},
				},
			},
		}
		return json.Marshal(body)
	} else if apiName == "queryFeatureList" {
		body := map[string]interface{}{
			"header": map[string]interface{}{
				"app_id": appID,
				"status": 3,
			},
			"parameter": map[string]interface{}{
				"s782b4996": map[string]interface{}{
					"func":    "queryFeatureList",
					"groupId": groupID,
					"queryFeatureListRes": map[string]interface{}{
						"encoding": "utf8",
						"compress": "raw",
						"format":   "json",
					},
				},
			},
		}
		return json.Marshal(body)
	} else if apiName == "searchFea" {
		// audioBytes, err := os.ReadFile(filePath)
		// if err != nil {
		// 	return nil, err
		// }
		body := map[string]interface{}{
			"header": map[string]interface{}{
				"app_id": appID,
				"status": 3,
			},
			"parameter": map[string]interface{}{
				"s782b4996": map[string]interface{}{
					"func":    "searchFea",
					"groupId": groupID,
					"topK":    topK,
					"searchFeaRes": map[string]interface{}{
						"encoding": "utf8",
						"compress": "raw",
						"format":   "json",
					},
				},
			},
			"payload": map[string]interface{}{
				"resource": map[string]interface{}{
					"encoding":    "lame",
					"sample_rate": 16000,
					"channels":    1,
					"bit_depth":   16,
					"status":      3,
					"audio":       audio,
					// "audio":       base64.StdEncoding.EncodeToString(audioBytes),
				},
			},
		}
		return json.Marshal(body)
	} else if apiName == "searchScoreFea" {
		// audioBytes, err := os.ReadFile(filePath)
		// if err != nil {
		// 	return nil, err
		// }
		body := map[string]interface{}{
			"header": map[string]interface{}{
				"app_id": appID,
				"status": 3,
			},
			"parameter": map[string]interface{}{
				"s782b4996": map[string]interface{}{
					"func":         "searchScoreFea",
					"groupId":      groupID,
					"dstFeatureId": featureID,
					"searchScoreFeaRes": map[string]interface{}{
						"encoding": "utf8",
						"compress": "raw",
						"format":   "json",
					},
				},
			},
			"payload": map[string]interface{}{
				"resource": map[string]interface{}{
					"encoding":    "lame",
					"sample_rate": 16000,
					"channels":    1,
					"bit_depth":   16,
					"status":      3,
					"audio":       audio,
					// "audio":       base64.StdEncoding.EncodeToString(audioBytes),
				},
			},
		}
		return json.Marshal(body)
	} else if apiName == "updateFeature" {
		// audioBytes, err := os.ReadFile(filePath)
		// if err != nil {
		// 	return nil, err
		// }
		body := map[string]interface{}{
			"header": map[string]interface{}{
				"app_id": appID,
				"status": 3,
			},
			"parameter": map[string]interface{}{
				"s782b4996": map[string]interface{}{
					"func":        "updateFeature",
					"groupId":     groupID,
					"featureId":   featureID,
					"featureInfo": featureInfo,
					"updateFeatureRes": map[string]interface{}{
						"encoding": "utf8",
						"compress": "raw",
						"format":   "json",
					},
				},
			},
			"payload": map[string]interface{}{
				"resource": map[string]interface{}{
					"encoding":    "lame",
					"sample_rate": 16000,
					"channels":    1,
					"bit_depth":   16,
					"status":      3,
					"audio":       audio,
					// "audio":       base64.StdEncoding.EncodeToString(audioBytes),
				},
			},
		}
		return json.Marshal(body)
	} else if apiName == "deleteGroup" {
		body := map[string]interface{}{
			"header": map[string]interface{}{
				"app_id": appID,
				"status": 3,
			},
			"parameter": map[string]interface{}{
				"s782b4996": map[string]interface{}{
					"func":    "deleteGroup",
					"groupId": groupID,
					"deleteGroupRes": map[string]interface{}{
						"encoding": "utf8",
						"compress": "raw",
						"format":   "json",
					},
				},
			},
		}
		return json.Marshal(body)
	}
	return nil, fmt.Errorf("invalid api name")
}

func reqURL(r *reqInfo) (*result, int, error) {
	apiName := r.apiName

	genReqURL := &GenReqURL{}
	body, err := genReqBody(r)
	if err != nil {
		return nil, -1, err
	}
	requestURL, err := genReqURL.assembleWSAuthURL("https://api.xf-yun.com/v1/private/s782b4996", r.apiKey, r.apiSecret, "POST")
	if err != nil {
		return nil, -1, err
	}
	log.Println(requestURL)

	headers := map[string]string{
		"content-type": "application/json",
		"host":         "api.xf-yun.com",
		"appid":        r.appId,
	}
	request, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, -1, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, -1, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, -1, err
	}
	var tempResult map[string]interface{}
	err = json.Unmarshal(responseBody, &tempResult)
	if err != nil {
		return nil, -1, err
	}
	log.Println(tempResult)

	fcode := tempResult["header"].(map[string]interface{})["code"].(float64)
	code := int(fcode)
	if code != 0 {
		err := errors.New(tempResult["header"].(map[string]interface{})["message"].(string))
		log.Println("code:", code, "message:", err.Error())
		return nil, code, err
	}

	subject := ""
	switch apiName {
	case "searchFea":
		subject = "searchFeaRes"
	case "createFeature":
		subject = "createFeatureRes"
	case "createGroup":
		subject = "createGroupRes"
	case "deleteFeature":
		subject = "deleteFeatureRes"
	case "queryFeatureList":
		subject = "queryFeatureListRes"
	case "searchScoreFea":
		subject = "searchScoreFeaRes"
	case "updateFeature":
		subject = "updateFeatureRes"
	case "deleteGroup":
		subject = "deleteGroupRes"
	default:
		return nil, -1, fmt.Errorf("invalid api name")
	}

	encodedText := tempResult["payload"].(map[string]interface{})[subject].(map[string]interface{})["text"].(string)
	decodedText, err := base64.StdEncoding.DecodeString(encodedText)
	if err != nil {
		return nil, -1, err
	}
	log.Println(string(decodedText))

	res := &result{}

	if apiName == "searchFea" {
		response := &SearchFeaResponse{}
		err := json.Unmarshal(decodedText, &response)
		if err != nil {
			log.Println("searchFee json decode error:", err)
			return nil, -1, err
		}
		res.featureId = response.ScoreList[0].FeatureId
		res.score = response.ScoreList[0].Score
	} else if apiName == "searchScoreFea" {
		response := &DearchScoreFeaResponse{}
		err := json.Unmarshal(decodedText, &response)
		if err != nil {
			log.Println("searchScoreFea json decode error:", err)
			return nil, -1, err
		}
		res.featureId = response.FeatureId
		res.score = response.Score
	}

	log.Println("result:", res.featureId, res.score)

	return res, 0, nil
}

// {
//   "score": 1,
//   "featureInfo": "iFLYTEK_examples_featureInfo",
//   "featureId": "iFLYTEK_examples_featureId"
// }

type DearchScoreFeaResponse struct {
	Score       float64 `json:"score"`
	FeatureInfo string  `json:"featureInfo"`
	FeatureId   string  `json:"featureId"`
}

// {
//   "scoreList": [
//     {
//       "score": 1,
//       "featureInfo": "iFLYTEK_examples_featureInfo1",
//       "featureId": "iFLYTEK_examples_featureId1"
//     },
//     {
//       "score": 0.85,
//       "featureInfo": "iFLYTEK_examples_featureInfo",
//       "featureId": "iFLYTEK_examples_featureId"
//     }
//   ]
// }

type SearchFeaResponse struct {
	ScoreList []DearchScoreFeaResponse `json:"scoreList"`
}

// func main() {
// 	appID := "f6e7d8fe"
// 	apiSecret := "YmRiZjA5OGE1NmJlZGJhMWFhZDBkMWFk"
// 	apiKey := "6c199ca711eb5007e52b5a13efe55c7b"
// 	flag.Parse()
// 	err := reqURL(*apiName, appID, apiKey, apiSecret, *filePath)
// 	if err != nil {
// 		log.Println(err)
// 		os.Exit(1)
// 	}
// }

type result struct {
	featureId string
	score     float64
}

func vrg(r *reqInfo) (*result, int, error) {
	r.appId = "f6e7d8fe"
	r.apiSecret = "YmRiZjA5OGE1NmJlZGJhMWFhZDBkMWFk"
	r.apiKey = "6c199ca711eb5007e52b5a13efe55c7b"
	r.topK = 1
	r.groupInfo = r.groupId
	r.groupName = r.groupId
	log.Println(r.apiName, r.featureId, r.featureInfo)

	return reqURL(r)
}
