package isuports

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

func TestUUIDEncodingAndEntropyFailure(t *testing.T) {
	for _, tc := range []struct {
		fill byte
		want string
	}{
		{0, "00000000-0000-4000-8000-000000000000"},
		{255, "ffffffff-ffff-4fff-bfff-ffffffffffff"},
	} {
		got, err := generateUUID(bytes.NewReader(bytes.Repeat([]byte{tc.fill}, 16)))
		if err != nil || got != tc.want {
			t.Fatalf("got %q, %v; want %q", got, err, tc.want)
		}
	}
	if id, err := generateUUID(bytes.NewReader(make([]byte, 15))); id != "" || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("partial entropy must not produce an ID: %q, %v", id, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if id, err := dispenseID(ctx); id != "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %q %v", id, err)
	}
}

func TestDispenseIDConcurrentWithoutDatabase(t *testing.T) {
	// No id_generator table or database connection is available to the generator.
	old := adminDB
	adminDB = nil
	defer func() { adminDB = old }()
	const workers, each = 16, 256
	ids := make(chan string, workers*each)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				id, err := dispenseID(context.Background())
				if err != nil {
					t.Error(err)
					return
				}
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)
	valid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for id := range ids {
		if !valid.MatchString(id) || seen[id] {
			t.Fatalf("invalid or duplicate UUID: %q", id)
		}
		seen[id] = true
	}
	if len(seen) != workers*each {
		t.Fatalf("generated %d IDs", len(seen))
	}
}

func TestUUIDLegacyIDsRankingAndAuthentication(t *testing.T) {
	ctx := context.Background()
	newID := func() string {
		t.Helper()
		id, err := dispenseID(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	playerID, competitionID := newID(), newID()
	db := newRankingTestDB(t)
	addRankingPlayer(t, db, "0001", "Legacy", false)
	addRankingPlayer(t, db, playerID, "UUID", false)
	if _, err := db.Exec("INSERT INTO competition VALUES (?, 1, 'UUID competition', NULL, 0, 0)", competitionID); err != nil {
		t.Fatal(err)
	}
	if c, err := retrieveCompetition(ctx, db, competitionID); err != nil || c.ID != competitionID {
		t.Fatalf("competition: %#v %v", c, err)
	}
	rows := []PlayerScoreRow{
		{ID: newID(), TenantID: 1, CompetitionID: competitionID, PlayerID: playerID, Score: 100, RowNum: 1},
		{ID: newID(), TenantID: 1, CompetitionID: competitionID, PlayerID: "0001", Score: 80, RowNum: 2},
		{ID: newID(), TenantID: 1, CompetitionID: competitionID, PlayerID: playerID, Score: 80, RowNum: 3},
	}
	if err := replaceCompetitionScores(ctx, db.DB, 1, competitionID, rows); err != nil {
		t.Fatal(err)
	}
	ranks, err := retrieveCompetitionRanks(ctx, db, 1, competitionID, 0)
	if err != nil || len(ranks) != 2 || ranks[0].PlayerID != "0001" || ranks[1].PlayerID != playerID || ranks[1].Score != 80 {
		t.Fatalf("mixed-ID latest-row / tie order: %#v %v", ranks, err)
	}
	// Exercise the real JWT parser and player authorization with both ID formats.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "public.pem")
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ISUCON_JWT_KEY_FILE", keyFile)
	t.Setenv("ISUCON_BASE_HOSTNAME", ".t.isucon.dev")
	if _, err := db.Exec(`CREATE TABLE tenant (id INTEGER PRIMARY KEY, name TEXT, display_name TEXT, created_at INTEGER, updated_at INTEGER);
 INSERT INTO tenant VALUES (1, 'isucon', 'ISUCON', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	old := adminDB
	adminDB = db.DB
	defer func() { adminDB = old }()
	for _, id := range []string{"0001", playerID} {
		token := jwt.New()
		for name, value := range map[string]interface{}{"sub": id, "aud": []string{"isucon"}, "role": RolePlayer, "exp": time.Now().Add(time.Hour)} {
			if err := token.Set(name, value); err != nil {
				t.Fatal(err)
			}
		}
		signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, key))
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "http://isucon.t.isucon.dev/api/player/player/"+id, nil)
		req.AddCookie(&http.Cookie{Name: cookieName, Value: string(signed)})
		viewer, err := parseViewer(echo.New().NewContext(req, httptest.NewRecorder()))
		if err != nil {
			t.Fatal(err)
		}
		if viewer.playerID != id || viewer.tenantID != 1 {
			t.Fatalf("viewer: %#v", viewer)
		}
		if err := authorizePlayer(ctx, db, viewer.playerID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("UPDATE player SET is_disqualified=1 WHERE id=?", playerID); err != nil {
		t.Fatal(err)
	}
	var httpErr *echo.HTTPError
	if err := authorizePlayer(ctx, db, playerID); !errors.As(err, &httpErr) || httpErr.Code != http.StatusForbidden {
		t.Fatalf("disqualified UUID: %v", err)
	}
}
