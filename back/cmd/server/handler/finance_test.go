package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"noted/internal/database"
	"noted/internal/database/generated"
	"noted/internal/service"

	"github.com/go-chi/chi/v5"
)

// newTestFinanceHandler mirrors router.go's finance route table.
func newTestFinanceHandler(t *testing.T) (*Handler, http.Handler, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "handler-finance.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	q := generated.New(db)
	wallets := service.NewWalletService(q)
	transactions := service.NewTransactionService(q, "IDR")
	h := New(nil, nil, nil, nil, wallets, transactions, false)

	r := chi.NewRouter()
	r.Get("/api/finance/balance", h.FinanceBalance)
	r.Get("/api/finance/wallets", h.ListWallets)
	r.Post("/api/finance/wallets", h.CreateWallet)
	r.Patch("/api/finance/wallets/{id}", h.UpdateWallet)
	r.Get("/api/finance/transactions", h.ListTransactions)
	r.Post("/api/finance/transactions", h.CreateTransaction)
	r.Post("/api/finance/transfers", h.CreateTransfer)
	r.Patch("/api/finance/transactions/{id}", h.UpdateTransaction)
	r.Delete("/api/finance/transactions/{id}", h.DeleteTransaction)
	return h, r, db
}

func decodeWallet(t *testing.T, rec *httptest.ResponseRecorder) walletJSON {
	t.Helper()
	var wallet walletJSON
	if err := json.NewDecoder(rec.Body).Decode(&wallet); err != nil {
		t.Fatalf("decode wallet: %v", err)
	}
	return wallet
}

func createWalletViaAPI(t *testing.T, router http.Handler, body string) walletJSON {
	t.Helper()
	rec := httpRequest(t, router, http.MethodPost, "/api/finance/wallets", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed wallet status = %d, body %s", rec.Code, rec.Body.String())
	}
	return decodeWallet(t, rec)
}

func createTransactionViaAPI(t *testing.T, router http.Handler, body string) transactionJSON {
	t.Helper()
	rec := httpRequest(t, router, http.MethodPost, "/api/finance/transactions", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed tx status = %d, body %s", rec.Code, rec.Body.String())
	}
	var tx transactionJSON
	if err := json.NewDecoder(rec.Body).Decode(&tx); err != nil {
		t.Fatalf("decode tx: %v", err)
	}
	return tx
}

// fieldIs asserts the 400 envelope carries the expected field message.
func fieldIs(t *testing.T, rec *httptest.ResponseRecorder, field, want string) bool {
	t.Helper()
	_, _, fields := decodeErr(t, rec)
	return fields[field] == want
}

func TestListWallets_ok_includesSeedCash(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	rec := httpRequest(t, router, http.MethodGet, "/api/finance/wallets", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body walletsListResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ID != service.WALLET_SEED_CASH_ID || body.Data[0].Name != "Cash" {
		t.Fatalf("data = %+v, want seeded Cash", body.Data)
	}
}

func TestListWallets_ok_archivedFlag(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)
	bank := createWalletViaAPI(t, router, `{"name":"Bank"}`)
	rec := httpRequest(t, router, http.MethodPatch, "/api/finance/wallets/"+bank.ID, `{"archived":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("archive status = %d", rec.Code)
	}

	active := httpRequest(t, router, http.MethodGet, "/api/finance/wallets", "")
	var activeBody walletsListResponse
	_ = json.NewDecoder(active.Body).Decode(&activeBody)
	if len(activeBody.Data) != 1 || activeBody.Data[0].Name != "Cash" {
		t.Fatalf("active = %+v, want only Cash", activeBody.Data)
	}

	all := httpRequest(t, router, http.MethodGet, "/api/finance/wallets?archived=1", "")
	var allBody walletsListResponse
	_ = json.NewDecoder(all.Body).Decode(&allBody)
	if len(allBody.Data) != 2 {
		t.Fatalf("all = %d wallets, want 2", len(allBody.Data))
	}
}

func TestCreateWallet_badRequests(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	empty := httpRequest(t, router, http.MethodPost, "/api/finance/wallets", `{"name":"  "}`)
	if empty.Code != http.StatusBadRequest || !fieldIs(t, empty, "name", "required") {
		t.Fatalf("empty name: code %d body %s", empty.Code, empty.Body.String())
	}

	unknown := httpRequest(t, router, http.MethodPost, "/api/finance/wallets", `{"archived":true}`)
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown key: code %d, want 400", unknown.Code)
	}
}

func TestUpdateWallet_renameAndArchive(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)
	bank := createWalletViaAPI(t, router, `{"name":"Bank"}`)

	renamed := httpRequest(t, router, http.MethodPatch, "/api/finance/wallets/"+bank.ID, `{"name":"Bank BCA"}`)
	if renamed.Code != http.StatusOK || decodeWallet(t, renamed).Name != "Bank BCA" {
		t.Fatalf("rename: code %d body %s", renamed.Code, renamed.Body.String())
	}

	archived := httpRequest(t, router, http.MethodPatch, "/api/finance/wallets/"+bank.ID, `{"archived":true}`)
	wallet := decodeWallet(t, archived)
	if archived.Code != http.StatusOK || wallet.ArchivedAt == nil {
		t.Fatalf("archive: code %d wallet %+v", archived.Code, wallet)
	}
}

func TestUpdateWallet_noFieldsAndMissing(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	none := httpRequest(t, router, http.MethodPatch, "/api/finance/wallets/"+service.WALLET_SEED_CASH_ID, `{}`)
	if none.Code != http.StatusBadRequest || !fieldIs(t, none, "name", "name or archived required") {
		t.Fatalf("no fields: code %d body %s", none.Code, none.Body.String())
	}

	missing := httpRequest(t, router, http.MethodPatch, "/api/finance/wallets/wal_nope", `{"name":"x"}`)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing: code %d, want 404", missing.Code)
	}
}

func TestFinanceBalance_ok_totalAndPerWallet(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)
	bank := createWalletViaAPI(t, router, `{"name":"Bank"}`)

	createTransactionViaAPI(t, router, `{"kind":"income","wallet_id":"`+service.WALLET_SEED_CASH_ID+`","amount_cents":200000,"occurred_on":"2026-10-18"}`)
	rec := httpRequest(t, router, http.MethodPost, "/api/finance/transfers",
		`{"from_wallet_id":"`+bank.ID+`","to_wallet_id":"`+service.WALLET_SEED_CASH_ID+`","amount_cents":100000,"occurred_on":"2026-10-19"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("transfer: code %d body %s", rec.Code, rec.Body.String())
	}

	balance := httpRequest(t, router, http.MethodGet, "/api/finance/balance", "")
	if balance.Code != http.StatusOK {
		t.Fatalf("balance: code %d", balance.Code)
	}
	var body balanceResponse
	if err := json.NewDecoder(balance.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Currency != "IDR" || body.Total.BalanceCents != 200000 {
		t.Fatalf("currency/total = %q/%d", body.Currency, body.Total.BalanceCents)
	}
	byID := map[string]int64{}
	for _, w := range body.Wallets {
		byID[w.WalletID] = w.BalanceCents
	}
	if byID[service.WALLET_SEED_CASH_ID] != 300000 || byID[bank.ID] != -100000 {
		t.Fatalf("wallets = %+v, want cash 300000 / bank -100000", byID)
	}
}

func TestListTransactions_ok_cursorRoundTrip(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)
	cash := service.WALLET_SEED_CASH_ID
	createTransactionViaAPI(t, router, `{"kind":"expense","wallet_id":"`+cash+`","amount_cents":1000,"occurred_on":"2026-10-01"}`)
	createTransactionViaAPI(t, router, `{"kind":"expense","wallet_id":"`+cash+`","amount_cents":1000,"occurred_on":"2026-10-02"}`)
	createTransactionViaAPI(t, router, `{"kind":"expense","wallet_id":"`+cash+`","amount_cents":1000,"occurred_on":"2026-10-03"}`)

	first := httpRequest(t, router, http.MethodGet, "/api/finance/transactions?limit=2", "")
	if first.Code != http.StatusOK {
		t.Fatalf("first page: code %d", first.Code)
	}
	var page1 transactionsListResponse
	_ = json.NewDecoder(first.Body).Decode(&page1)
	if len(page1.Data) != 2 || page1.NextCursor == nil {
		t.Fatalf("page1 = %d rows, cursor %v", len(page1.Data), page1.NextCursor)
	}
	if page1.Data[0].OccurredOn != "2026-10-03" {
		t.Fatalf("newest first? got %s", page1.Data[0].OccurredOn)
	}

	second := httpRequest(t, router, http.MethodGet, "/api/finance/transactions?limit=2&cursor="+*page1.NextCursor, "")
	var page2 transactionsListResponse
	_ = json.NewDecoder(second.Body).Decode(&page2)
	if len(page2.Data) != 1 || page2.NextCursor != nil {
		t.Fatalf("page2 = %d rows, cursor %v, want 1 row and null", len(page2.Data), page2.NextCursor)
	}
}

func TestListTransactions_badLimitAndCursor_400(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	zero := httpRequest(t, router, http.MethodGet, "/api/finance/transactions?limit=0", "")
	if zero.Code != http.StatusBadRequest || !fieldIs(t, zero, "limit", "must be between 1 and 200") {
		t.Fatalf("limit 0: code %d body %s", zero.Code, zero.Body.String())
	}

	huge := httpRequest(t, router, http.MethodGet, "/api/finance/transactions?limit=201", "")
	if huge.Code != http.StatusBadRequest {
		t.Fatalf("limit 201: code %d, want 400", huge.Code)
	}

	bad := httpRequest(t, router, http.MethodGet, "/api/finance/transactions?cursor=@@@@", "")
	if bad.Code != http.StatusBadRequest || !fieldIs(t, bad, "cursor", "malformed") {
		t.Fatalf("bad cursor: code %d body %s", bad.Code, bad.Body.String())
	}
}

func TestListTransactions_walletFilter(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)
	bank := createWalletViaAPI(t, router, `{"name":"Bank"}`)

	createTransactionViaAPI(t, router, `{"kind":"expense","wallet_id":"`+service.WALLET_SEED_CASH_ID+`","amount_cents":1000,"occurred_on":"2026-10-01"}`)
	rec := httpRequest(t, router, http.MethodPost, "/api/finance/transfers",
		`{"from_wallet_id":"`+bank.ID+`","to_wallet_id":"`+service.WALLET_SEED_CASH_ID+`","amount_cents":500,"occurred_on":"2026-10-02"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("transfer: %d %s", rec.Code, rec.Body.String())
	}

	bankPage := httpRequest(t, router, http.MethodGet, "/api/finance/transactions?wallet_id="+bank.ID, "")
	var body transactionsListResponse
	_ = json.NewDecoder(bankPage.Body).Decode(&body)
	if len(body.Data) != 1 || body.Data[0].Kind != "transfer" {
		t.Fatalf("bank rows = %+v, want the transfer side", body.Data)
	}
}

func TestCreateTransaction_badRequests(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	cases := []struct {
		name  string
		body  string
		field string
		want  string
	}{
		{"transfer kind", `{"kind":"transfer","wallet_id":"wal_cash","amount_cents":100,"occurred_on":"2026-10-01"}`, "kind", "must be income or expense (use /transfers)"},
		{"missing wallet", `{"kind":"expense","wallet_id":"","amount_cents":100,"occurred_on":"2026-10-01"}`, "wallet_id", "required"},
		{"bad date", `{"kind":"expense","wallet_id":"wal_cash","amount_cents":100,"occurred_on":"01/10/2026"}`, "occurred_on", "must be YYYY-MM-DD"},
		{"zero amount", `{"kind":"expense","wallet_id":"wal_cash","amount_cents":0,"occurred_on":"2026-10-01"}`, "amount_cents", "must be at least 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httpRequest(t, router, http.MethodPost, "/api/finance/transactions", tc.body)
			if rec.Code != http.StatusBadRequest || !fieldIs(t, rec, tc.field, tc.want) {
				t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateTransaction_unknownKey_400(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	rec := httpRequest(t, router, http.MethodPost, "/api/finance/transactions",
		`{"kind":"expense","wallet_id":"wal_cash","amount_cents":100,"occurred_on":"2026-10-01","currency":"USD"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code %d, want 400 (currency is server-stamped)", rec.Code)
	}
}

func TestCreateTransfer_badRequests(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	same := httpRequest(t, router, http.MethodPost, "/api/finance/transfers",
		`{"from_wallet_id":"wal_cash","to_wallet_id":"wal_cash","amount_cents":100,"occurred_on":"2026-10-01"}`)
	if same.Code != http.StatusBadRequest || !fieldIs(t, same, "to_wallet_id", "must differ from from_wallet_id") {
		t.Fatalf("same wallet: code %d body %s", same.Code, same.Body.String())
	}

	unknown := httpRequest(t, router, http.MethodPost, "/api/finance/transfers",
		`{"from_wallet_id":"wal_cash","to_wallet_id":"wal_nope","amount_cents":100,"occurred_on":"2026-10-01"}`)
	if unknown.Code != http.StatusBadRequest || !fieldIs(t, unknown, "to_wallet_id", "unknown wallet") {
		t.Fatalf("unknown: code %d body %s", unknown.Code, unknown.Body.String())
	}
}

func TestUpdateTransaction_okAndKindRejected(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)
	tx := createTransactionViaAPI(t, router,
		`{"kind":"expense","wallet_id":"`+service.WALLET_SEED_CASH_ID+`","amount_cents":1000,"occurred_on":"2026-10-01"}`)

	patched := httpRequest(t, router, http.MethodPatch, "/api/finance/transactions/"+tx.ID, `{"amount_cents":2500}`)
	if patched.Code != http.StatusOK {
		t.Fatalf("patch: code %d body %s", patched.Code, patched.Body.String())
	}
	var updated transactionJSON
	_ = json.NewDecoder(patched.Body).Decode(&updated)
	if updated.AmountCents != 2500 || updated.Kind != "expense" {
		t.Fatalf("updated = %+v", updated)
	}

	kind := httpRequest(t, router, http.MethodPatch, "/api/finance/transactions/"+tx.ID, `{"kind":"transfer"}`)
	if kind.Code != http.StatusBadRequest {
		t.Fatalf("kind patch: code %d, want 400 (immutable, unknown field)", kind.Code)
	}

	missing := httpRequest(t, router, http.MethodPatch, "/api/finance/transactions/tx_nope", `{}`)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing: code %d, want 404", missing.Code)
	}
}

func TestDeleteTransaction_204AndGone(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)
	tx := createTransactionViaAPI(t, router,
		`{"kind":"expense","wallet_id":"`+service.WALLET_SEED_CASH_ID+`","amount_cents":1000,"occurred_on":"2026-10-01"}`)

	rec := httpRequest(t, router, http.MethodDelete, "/api/finance/transactions/"+tx.ID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: code %d", rec.Code)
	}

	list := httpRequest(t, router, http.MethodGet, "/api/finance/transactions", "")
	var body transactionsListResponse
	_ = json.NewDecoder(list.Body).Decode(&body)
	if len(body.Data) != 0 {
		t.Fatalf("rows after void = %d, want 0", len(body.Data))
	}
}

// --- error exits: each drops exactly what its service call reads ---

func dropTable(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.Exec(stmt); err != nil {
		t.Fatalf("exec %q: %v", stmt, err)
	}
}

func TestListWallets_internal_500Envelope(t *testing.T) {
	_, router, db := newTestFinanceHandler(t)
	dropTable(t, db, `DROP TABLE wallets`)

	rec := httpRequest(t, router, http.MethodGet, "/api/finance/wallets", "")
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

func TestUpdateWallet_decodeError_400(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	rec := httpRequest(t, router, http.MethodPatch, "/api/finance/wallets/"+service.WALLET_SEED_CASH_ID, `{"nope":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code %d, want 400", rec.Code)
	}
}

func TestUpdateWallet_archiveMissing_404(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	rec := httpRequest(t, router, http.MethodPatch, "/api/finance/wallets/wal_nope", `{"archived":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code %d, want 404", rec.Code)
	}
}

func TestFinanceBalance_internal_500Envelope(t *testing.T) {
	_, router, db := newTestFinanceHandler(t)
	dropTable(t, db, `DROP TABLE transactions`)

	rec := httpRequest(t, router, http.MethodGet, "/api/finance/balance", "")
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

func TestListTransactions_internal_500Envelope(t *testing.T) {
	_, router, db := newTestFinanceHandler(t)
	dropTable(t, db, `DROP TABLE transactions`)

	rec := httpRequest(t, router, http.MethodGet, "/api/finance/transactions", "")
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

func TestCreateTransfer_decodeError_400(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	rec := httpRequest(t, router, http.MethodPost, "/api/finance/transfers", `{"nope":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code %d, want 400", rec.Code)
	}
}

func TestDeleteTransaction_internal_500Envelope(t *testing.T) {
	_, router, db := newTestFinanceHandler(t)
	dropTable(t, db, `DROP TABLE transactions`)

	rec := httpRequest(t, router, http.MethodDelete, "/api/finance/transactions/tx_x", "")
	status, code, _ := decodeErr(t, rec)
	if status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("status=%d code=%q, want 500 internal_error", status, code)
	}
}

func TestListTransactions_cursorWithoutSeparator_400(t *testing.T) {
	_, router, _ := newTestFinanceHandler(t)

	rec := httpRequest(t, router, http.MethodGet, "/api/finance/transactions?cursor=Z2FyYmFnZQ", "")
	if rec.Code != http.StatusBadRequest || !fieldIs(t, rec, "cursor", "malformed") {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
}
