package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTunnelRoutesHappyPath(t *testing.T) {
	router, token := newProtectedRouter(t)

	createResp := performJSON(t, router, http.MethodPost, "/api/tunnels", token, map[string]any{
		"name":     "proxy-a",
		"group_id": "group-asia",
		"username": "u1",
		"password": "p1",
	})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/tunnels status = %d, want 201", createResp.Code)
	}

	var created map[string]any
	_ = json.Unmarshal(createResp.Body.Bytes(), &created)
	tunnelID := created["id"].(string)

	listResp := perform(t, router, http.MethodGet, "/api/tunnels", token, nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("GET /api/tunnels status = %d, want 200", listResp.Code)
	}

	getResp := perform(t, router, http.MethodGet, "/api/tunnels/"+tunnelID, token, nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("GET /api/tunnels/:id status = %d, want 200", getResp.Code)
	}

	updateResp := performJSON(t, router, http.MethodPut, "/api/tunnels/"+tunnelID, token, map[string]any{
		"name":     "proxy-b",
		"group_id": "group-us",
		"username": "u2",
		"password": "p2",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("PUT /api/tunnels/:id status = %d, want 200", updateResp.Code)
	}

	refreshResp := performJSON(t, router, http.MethodPost, "/api/tunnels/"+tunnelID+"/refresh", token, map[string]any{})
	if refreshResp.Code != http.StatusOK {
		t.Fatalf("POST /api/tunnels/:id/refresh status = %d, want 200", refreshResp.Code)
	}

	eventsResp := perform(t, router, http.MethodGet, "/api/tunnels/"+tunnelID+"/events", token, nil)
	if eventsResp.Code != http.StatusOK {
		t.Fatalf("GET /api/tunnels/:id/events status = %d, want 200", eventsResp.Code)
	}

	stopResp := performJSON(t, router, http.MethodPost, "/api/tunnels/"+tunnelID+"/stop", token, map[string]any{})
	if stopResp.Code != http.StatusOK {
		t.Fatalf("POST /api/tunnels/:id/stop status = %d, want 200", stopResp.Code)
	}

	startResp := performJSON(t, router, http.MethodPost, "/api/tunnels/"+tunnelID+"/start", token, map[string]any{})
	if startResp.Code != http.StatusOK {
		t.Fatalf("POST /api/tunnels/:id/start status = %d, want 200", startResp.Code)
	}

	deleteResp := perform(t, router, http.MethodDelete, "/api/tunnels/"+tunnelID, token, nil)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/tunnels/:id status = %d, want 204", deleteResp.Code)
	}
}

func TestTunnelRoutesAllowCrossGroupSameNameAndRejectSameGroupDuplicate(t *testing.T) {
	router, token := newProtectedRouter(t)

	firstResp := performJSON(t, router, http.MethodPost, "/api/tunnels", token, map[string]any{
		"name":     "shared",
		"group_id": "group-asia",
	})
	if firstResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/tunnels first status = %d, want 201", firstResp.Code)
	}

	secondResp := performJSON(t, router, http.MethodPost, "/api/tunnels", token, map[string]any{
		"name":     "shared",
		"group_id": "group-us",
	})
	if secondResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/tunnels second status = %d, want 201", secondResp.Code)
	}

	duplicateResp := performJSON(t, router, http.MethodPost, "/api/tunnels", token, map[string]any{
		"name":     "shared",
		"group_id": "group-asia",
	})
	if duplicateResp.Code != http.StatusConflict {
		t.Fatalf("POST /api/tunnels duplicate status = %d, want 409", duplicateResp.Code)
	}
}
