package main

import (
	"crypto/tls"
	"encoding/json"
	"main/ynison"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const AUTH_HEADER = "Authorization"

var client = new(http.Client)

func main() {
	// ya captcha bypass
	mTLSConfig := &tls.Config{
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}
	mTLSConfig.MinVersion = tls.VersionTLS11
	mTLSConfig.MaxVersion = tls.VersionTLS13

	tr := &http.Transport{
		TLSClientConfig: mTLSConfig,
	}
	client.Transport = tr
	client.Timeout = 5 * time.Second
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	r.GET("/get_current_track_beta", tokenGIN)
	r.Run(":8080")
}

func tokenGIN(c *gin.Context) {
	token := c.Request.Header.Get("ya-token")
	tokenHeader := c.Request.Header.Get(AUTH_HEADER)
	if token == "" && tokenHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"place": "early",
			"error": "token not found",
		})
		return
	}
	if token != "" {
		tokenHeader = "OAuth " + token
	}

	// канал, куда будем отправлять данные по готовности
	done := make(chan ynison.PutYnisonStateResponse, 1)

	y := ynison.NewClient(tokenHeader)
	defer y.Close()
	y.OnMessage(func(pysr ynison.PutYnisonStateResponse) {
		done <- pysr
	})

	err := y.Connect()
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"place": "ynison connect",
			"error": err.Error(),
		})
		return
	}

	select {
	case data := <-done:
		if len(data.PlayerState.PlayerQueue.PlayableList) > 0 {
			index := data.PlayerState.PlayerQueue.CurrentPlayableIndex
			trackID := data.PlayerState.PlayerQueue.PlayableList[index].PlayableID
			track, err := trackdata(trackID, tokenHeader)
			if err != nil {
				c.JSON(http.StatusTeapot, gin.H{
					"place": "get track",
					"error": err.Error(),
				})
				return
			}
			artists := []string{}
			for _, artist := range track.Result[0].Artists {
				artists = append(artists, artist.Name)
			}
			trackIDInt, _ := strconv.Atoi(trackID)
			c.JSON(http.StatusOK, gin.H{
				"paused":      data.PlayerState.Status.Paused,
				"duration_ms": data.PlayerState.Status.DurationMs,
				"progress_ms": data.PlayerState.Status.ProgressMs,
				"entity_id":   data.PlayerState.PlayerQueue.EntityID,
				"entity_type": data.PlayerState.PlayerQueue.EntityType,
				"track": gin.H{
					"track_id":      trackIDInt,
					"title":         track.Result[0].Title,
					"artist":        artists,
					"img":           "https://" + strings.Replace(track.Result[0].CoverURI, "%%", "1000x1000", 1),
					"duration":      track.Result[0].DurationMs / 1000,
					"download-link": "https://ident.me",
				},
			})
			return
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "PlayerQueue information missing",
			})
			return
		}
	case <-time.After(10 * time.Second):
		c.JSON(http.StatusGatewayTimeout, gin.H{
			"place": "10 seconds timeout",
			"error": "Failed to retrieve data",
		})
		return
	case <-c.Request.Context().Done():
		c.JSON(http.StatusGatewayTimeout, gin.H{
			"place": "context.Done",
			"error": "context.Done",
		})
		return
	}
}

// информация о треке
func trackdata(trackid, header string) (*trackresponse, error) {
	req, err := http.NewRequest("GET", "https://api.music.yandex.net/tracks/"+trackid, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", header)
	req.Header.Add("x-Yandex-Music-Client", "YandexMusicAndroid/24024312")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", "okhttp/4.12.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, err
	}
	data := new(trackresponse)
	err = json.NewDecoder(resp.Body).Decode(data)
	return data, err
}

type trackresponse struct {
	Result []struct {
		Title      string `json:"title"`
		DurationMs int    `json:"durationMs"`
		Artists    []struct {
			Name string `json:"name"`
		} `json:"artists"`
		CoverURI string `json:"coverUri"`
	} `json:"result"`
}
