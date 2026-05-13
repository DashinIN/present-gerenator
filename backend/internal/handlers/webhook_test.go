package handlers

import "testing"

func TestParseKieCallbackFlatResultJSON(t *testing.T) {
	payload, err := parseKieCallback([]byte(`{
		"taskId":"task-1",
		"state":"success",
		"resultJson":"{\"resultUrls\":[\"https://example.com/a.png\"]}"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if payload.TaskID != "task-1" || payload.State != "success" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if len(payload.ResultURLs) != 1 || payload.ResultURLs[0] != "https://example.com/a.png" {
		t.Fatalf("unexpected urls: %#v", payload.ResultURLs)
	}
}

func TestParseKieCallbackNestedMediaFormat(t *testing.T) {
	payload, err := parseKieCallback([]byte(`{
		"code":200,
		"msg":"success",
		"data":{
			"task_id":"task-2",
			"info":{"result_urls":["https://example.com/b.png"]}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if payload.TaskID != "task-2" || payload.State != "success" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if len(payload.ResultURLs) != 1 || payload.ResultURLs[0] != "https://example.com/b.png" {
		t.Fatalf("unexpected urls: %#v", payload.ResultURLs)
	}
}
