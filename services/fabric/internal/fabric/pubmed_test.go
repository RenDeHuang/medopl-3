package fabric

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPubMedQueryNormalizesArticlesAndRecordsRedactedOperationEvidence(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Query().Get("api_key") != "" || r.URL.Query().Get("email") != "" {
			t.Fatalf("credentials sent to NCBI: %s", r.URL.RawQuery)
		}
		switch r.URL.Path {
		case "/esearch.fcgi":
			if r.URL.Query().Get("term") != "single cell" || r.URL.Query().Get("retstart") != "2" || r.URL.Query().Get("retmax") != "2" {
				t.Fatalf("search query = %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"esearchresult": map[string]any{"count": "3", "idlist": []string{"123", "456"}}})
		case "/efetch.fcgi":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<PubmedArticleSet><PubmedArticle><MedlineCitation><PMID>123</PMID><Article><ArticleTitle>Useful study</ArticleTitle><AuthorList><Author><LastName>Ng</LastName><Initials>A</Initials></Author><Author><CollectiveName>Study Group</CollectiveName></Author></AuthorList><Journal><Title>Science Journal</Title><JournalIssue><PubDate><Year>2025</Year></PubDate></JournalIssue></Journal></Article></MedlineCitation><PubmedData><ArticleIdList><ArticleId IdType="doi">10.1000/example</ArticleId></ArticleIdList></PubmedData></PubmedArticle></PubmedArticleSet>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := NewMemoryOperationStore()
	service := NewServiceWithPubMed(testProvider{}, store, server.Client(), server.URL)
	result, err := service.QueryPubMed(context.Background(), "1.0.0", PubMedQuery{Query: "single cell", Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 || result.Total != 3 || result.Page != 2 || len(result.Articles) != 1 {
		t.Fatalf("result = %#v requests=%d", result, requests.Load())
	}
	article := result.Articles[0]
	if article.PMID != "123" || article.Title != "Useful study" || strings.Join(article.Authors, ",") != "Ng A,Study Group" || article.Journal != "Science Journal" || article.Year != "2025" || article.DOI != "10.1000/example" || article.URL != "https://pubmed.ncbi.nlm.nih.gov/123/" {
		t.Fatalf("article = %#v", article)
	}
	operations, err := service.ListOperations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	last := operations[len(operations)-1]
	evidence, _ := json.Marshal(last.RedactedProviderPayload)
	if last.Action != "query_pubmed" || last.ResourceKind != "connector_version" || last.ResourceID != "pubmed@1.0.0" || last.Status != "succeeded" || !strings.Contains(string(evidence), "querySha256") || strings.Contains(string(evidence), "Useful study") || strings.Contains(string(evidence), "single cell") {
		t.Fatalf("operation evidence = %#v", last)
	}
}

func TestPubMedRetriesBoundedRetryAfterAndRejectsUnboundedInput(t *testing.T) {
	var attempts atomic.Int32
	var slept []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt < 3 {
			w.Header().Set("Retry-After", "9999")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"esearchresult": map[string]any{"count": "0", "idlist": []string{}}})
	}))
	defer server.Close()

	service := NewServiceWithPubMed(testProvider{}, NewMemoryOperationStore(), server.Client(), server.URL)
	service.pubmed.sleep = func(_ context.Context, delay time.Duration) error { slept = append(slept, delay); return nil }
	result, err := service.QueryPubMed(context.Background(), "1.0.0", PubMedQuery{Query: "bounded", Page: 1, PageSize: 20})
	if err != nil || result.Total != 0 || attempts.Load() != 3 || len(slept) != 2 {
		t.Fatalf("result=%#v err=%v attempts=%d slept=%v", result, err, attempts.Load(), slept)
	}
	for _, delay := range slept {
		if delay > 2*time.Second {
			t.Fatalf("unbounded Retry-After delay %s", delay)
		}
	}
	if _, err := service.QueryPubMed(context.Background(), "1.0.0", PubMedQuery{Query: strings.Repeat("x", 501), Page: 1, PageSize: 20}); err != ErrInvalidPubMedQuery {
		t.Fatalf("long query error = %v", err)
	}
	if _, err := service.QueryPubMed(context.Background(), "1.0.0", PubMedQuery{Query: "x", Page: 0, PageSize: 101}); err != ErrInvalidPubMedQuery {
		t.Fatalf("large page error = %v", err)
	}
}

func TestPubMedRejectsRetstartBeyondNCBILimit(t *testing.T) {
	service := NewServiceWithPubMed(testProvider{}, NewMemoryOperationStore(), nil, "https://example.invalid")
	if _, err := service.QueryPubMed(context.Background(), "1.0.0", PubMedQuery{Query: "x", Page: 101, PageSize: 100}); !errors.Is(err, ErrInvalidPubMedQuery) {
		t.Fatalf("retstart error = %v", err)
	}
}

func TestPubMedUsesOneDeadlineAndContextAwareBackoff(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	service := NewServiceWithPubMed(testProvider{}, NewMemoryOperationStore(), server.Client(), server.URL)
	service.pubmed.timeout = 30 * time.Millisecond
	started := time.Now()
	_, err := service.QueryPubMed(context.Background(), "1.0.0", PubMedQuery{Query: "deadline", Page: 1, PageSize: 20})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second || attempts.Load() != 1 {
		t.Fatalf("err=%v elapsed=%s attempts=%d", err, time.Since(started), attempts.Load())
	}
}

func TestPubMedDeadlineCoversSearchAndFetchTogether(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		if r.URL.Path == "/esearch.fcgi" {
			_ = json.NewEncoder(w).Encode(map[string]any{"esearchresult": map[string]any{"count": "1", "idlist": []string{"123"}}})
			return
		}
		_, _ = w.Write([]byte(`<PubmedArticleSet/>`))
	}))
	defer server.Close()

	service := NewServiceWithPubMed(testProvider{}, NewMemoryOperationStore(), server.Client(), server.URL)
	service.pubmed.timeout = 30 * time.Millisecond
	if _, err := service.QueryPubMed(context.Background(), "1.0.0", PubMedQuery{Query: "deadline", Page: 1, PageSize: 20}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shared deadline error = %v", err)
	}
}

type failingTerminalOperationStore struct {
	*MemoryOperationStore
	err error
}

func (s *failingTerminalOperationStore) Append(ctx context.Context, operation FabricOperation) error {
	if operation.Status == "failed" {
		return s.err
	}
	return s.MemoryOperationStore.Append(ctx, operation)
}

func TestPubMedReturnsProviderAndTerminalEvidenceErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	evidenceErr := errors.New("terminal_evidence_write_failed")
	store := &failingTerminalOperationStore{MemoryOperationStore: NewMemoryOperationStore(), err: evidenceErr}
	service := NewServiceWithPubMed(testProvider{}, store, server.Client(), server.URL)
	_, err := service.QueryPubMed(context.Background(), "1.0.0", PubMedQuery{Query: "failure", Page: 1, PageSize: 20})
	if !errors.Is(err, evidenceErr) || !strings.Contains(err.Error(), "pubmed_http_status_400") {
		t.Fatalf("joined error = %v", err)
	}
}

func TestRetryAfterHTTPDateIsBounded(t *testing.T) {
	future := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
	if delay := boundedRetryAfter(future); delay != 2*time.Second {
		t.Fatalf("bounded HTTP-date delay = %s", delay)
	}
}
