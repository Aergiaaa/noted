package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"noted/internal/auth"
)

// getWithSession issues an authenticated GET against the app's router.
func getWithSession(t *testing.T, h http.Handler, path, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: auth.SESSION_COOKIE, Value: cookie})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestFinanceFlow_endToEnd walks the F8 critical path through the real
// router: the DOMAIN.md "Buy keyboard" day (task + event + expense in
// Cash) plus a Bank → Cash transfer, then the derived balance —
// per-wallet math exact, the transfer cancels in the total (so the
// expense aggregation excludes it), and the wallet filter matches both
// sides of a transfer.
func TestFinanceFlow_endToEnd(t *testing.T) {
	app := newTestApp(t, Config{TOTPEncKey: cliEncKey, CurrencyDefault: "IDR"})
	router := app.newRouter()

	secret, _ := hostEnroll(t, app)
	live, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	login := postAuth(t, router, "/api/auth/login", `{"code":"`+live+`"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login = %d, want 200", login.Code)
	}
	cookie := sessionCookie(t, login)

	create := func(path, body string) string {
		t.Helper()
		rec := postAuth(t, router, path, body, cookie)
		if rec.Code != http.StatusCreated {
			t.Fatalf("POST %s = %d, want 201: %s", path, rec.Code, rec.Body.String())
		}
		var out struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		return out.ID
	}

	eventID := create("/api/events",
		`{"title":"Buy keyboard","starts_at":"2026-09-20T10:00:00Z","ends_at":"2026-09-20T11:00:00Z"}`)
	taskID := create("/api/tasks",
		`{"title":"Buy keyboard","due_date":"2026-09-20","event_id":"`+eventID+`"}`)
	bankID := create("/api/finance/wallets", `{"name":"Bank"}`)

	create("/api/finance/transactions",
		`{"kind":"expense","wallet_id":"wal_cash","amount_cents":500000,`+
			`"occurred_on":"2026-09-20","memo":"mechanical",`+
			`"task_id":"`+taskID+`","event_id":"`+eventID+`"}`)
	create("/api/finance/transfers",
		`{"from_wallet_id":"`+bankID+`","to_wallet_id":"wal_cash",`+
			`"amount_cents":1000000,"occurred_on":"2026-09-20","memo":"cash top-up"}`)

	rec := getWithSession(t, router, "/api/finance/balance", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("balance = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var balance struct {
		Currency string `json:"currency"`
		Total    struct {
			BalanceCents int64 `json:"balance_cents"`
		} `json:"total"`
		Wallets []struct {
			WalletID     string `json:"wallet_id"`
			BalanceCents int64  `json:"balance_cents"`
		} `json:"wallets"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&balance); err != nil {
		t.Fatalf("decode balance: %v", err)
	}
	balances := map[string]int64{}
	for _, w := range balance.Wallets {
		balances[w.WalletID] = w.BalanceCents
	}

	if balance.Currency != "IDR" {
		t.Errorf("currency = %q, want IDR (server-stamped)", balance.Currency)
	}
	// Cash: −500000 expense + 1000000 transfer in.
	if balances["wal_cash"] != 500000 {
		t.Errorf("cash balance = %d, want 500000", balances["wal_cash"])
	}
	// Bank: −1000000 transfer out (overdraft allowed).
	if balances[bankID] != -1000000 {
		t.Errorf("bank balance = %d, want -1000000", balances[bankID])
	}
	// Total = income − expense; the transfer cancels, so it is excluded
	// from the expense aggregation.
	if balance.Total.BalanceCents != -500000 {
		t.Errorf("total = %d, want -500000", balance.Total.BalanceCents)
	}

	// The wallet filter matches either side of a transfer.
	history := func(walletID string) int {
		t.Helper()
		rec := getWithSession(t, router, "/api/finance/transactions?wallet_id="+walletID, cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("history = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var page struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
			t.Fatalf("decode history: %v", err)
		}
		return len(page.Data)
	}
	if n := history("wal_cash"); n != 2 {
		t.Errorf("cash history = %d rows, want 2 (expense + transfer)", n)
	}
	if n := history(bankID); n != 1 {
		t.Errorf("bank history = %d rows, want 1 (transfer)", n)
	}
}
