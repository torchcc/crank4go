package connector

import (
	"io"
	"testing"
)

func TestClient(t *testing.T) {
	client := GetHttpClient()
	resp, err := client.Get("https://localhost:8443")
	if err != nil {
		t.Errorf("failed, err: %s", err.Error())
		return
	}
	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("failed, err: %s", err.Error())
		return
	}
	t.Log(string(bytes))

}
