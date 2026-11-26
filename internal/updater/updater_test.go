package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompareSemver(t *testing.T) {
	if got := CompareSemver("1.1.0", "v1.2.0"); got != -1 {
		t.Fatalf("expected -1, got %d", got)
	}
	if got := CompareSemver("v1.2.0", "1.2.0"); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := CompareSemver("1.3.0", "1.2.9"); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

func TestCheckLatestReleaseMockServer(t *testing.T) {
	// 1. Update available + SHA256SUMS asset
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/test/netpulse/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v1.3.0",
			"html_url": "https://github.com/test/netpulse/releases/tag/v1.3.0",
			"published_at": "2026-12-01T12:00:00Z",
			"body": "Mejoras de estabilidad",
			"assets": [
				{"name": "SHA256SUMS.txt", "browser_download_url": "https://github.com/test/netpulse/releases/download/v1.3.0/SHA256SUMS.txt"}
			]
		}`))
	}))
	defer srv.Close()

	res := CheckLatestRelease(context.Background(), "1.2.0", "test/netpulse", srv.URL)
	if res.Status != "update_available" || !res.UpdateAvailable || res.LatestVersion != "1.3.0" {
		t.Fatalf("unexpected update result: %+v", res)
	}
	if res.ChecksumsURL == "" || res.ReleaseURL == "" {
		t.Fatalf("expected URLs to be populated: %+v", res)
	}

	// 2. Up to date
	upToDate := CheckLatestRelease(context.Background(), "1.3.0", "test/netpulse", srv.URL)
	if upToDate.Status != "up_to_date" || upToDate.UpdateAvailable {
		t.Fatalf("expected up_to_date, got %+v", upToDate)
	}

	// 3. 404 / unavailable
	missing := CheckLatestRelease(context.Background(), "1.2.0", "other/missing", srv.URL)
	if missing.Status != "unavailable" || missing.Error == "" {
		t.Fatalf("expected unavailable status with error, got %+v", missing)
	}
}
