package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestGroupRoutesHappyPath(t *testing.T) {
	router, token := newProtectedRouter(t)

	createResp := performJSON(t, router, http.MethodPost, "/api/groups", token, map[string]any{
		"name":         "亚洲组",
		"filter_regex": "^(HK|JP)-",
		"description":  "test group",
	})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/groups status = %d, want 201", createResp.Code)
	}

	var created map[string]any
	_ = json.Unmarshal(createResp.Body.Bytes(), &created)
	groupID := created["id"].(string)

	listResp := perform(t, router, http.MethodGet, "/api/groups", token, nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("GET /api/groups status = %d, want 200", listResp.Code)
	}

	getResp := perform(t, router, http.MethodGet, "/api/groups/"+groupID, token, nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("GET /api/groups/:id status = %d, want 200", getResp.Code)
	}

	updateResp := performJSON(t, router, http.MethodPut, "/api/groups/"+groupID, token, map[string]any{
		"name":         "美区组",
		"filter_regex": "^US-",
		"description":  "updated",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("PUT /api/groups/:id status = %d, want 200", updateResp.Code)
	}

	membersResp := perform(t, router, http.MethodGet, "/api/groups/"+groupID+"/members", token, nil)
	if membersResp.Code != http.StatusOK {
		t.Fatalf("GET /api/groups/:id/members status = %d, want 200", membersResp.Code)
	}

	deleteResp := perform(t, router, http.MethodDelete, "/api/groups/"+groupID, token, nil)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/groups/:id status = %d, want 204", deleteResp.Code)
	}
}

func TestGroupRoutesRejectInvalidRegex(t *testing.T) {
	router, token := newProtectedRouter(t)

	resp := performJSON(t, router, http.MethodPost, "/api/groups", token, map[string]any{
		"name":         "坏组",
		"filter_regex": "[",
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/groups invalid regex status = %d, want 400", resp.Code)
	}
}
