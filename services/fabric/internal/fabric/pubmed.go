package fabric

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type pubMedClient struct {
	client  *http.Client
	baseURL string
	sleep   func(time.Duration)
}

func newPubMedClient(client *http.Client, baseURL string) *pubMedClient {
	if client == nil {
		client = &http.Client{}
	}
	return &pubMedClient{client: client, baseURL: strings.TrimRight(baseURL, "/"), sleep: time.Sleep}
}

type pubMedEvidence struct {
	QuerySHA256 string
	Page        int
	PageSize    int
	ResultCount int
	PMIDs       []string
}

func (s *Service) QueryPubMed(ctx context.Context, version string, input PubMedQuery) (PubMedResult, error) {
	input.Query = strings.TrimSpace(input.Query)
	if len(input.Query) == 0 || len(input.Query) > 500 || input.Page < 1 || input.Page > 1000 || input.PageSize < 1 || input.PageSize > 100 {
		return PubMedResult{}, ErrInvalidPubMedQuery
	}
	connector, err := s.Connector(ctx, "pubmed", version)
	if err != nil {
		return PubMedResult{}, err
	}
	if connector.Status != "approved" || !connector.ReadOnly {
		return PubMedResult{}, ErrCatalogRecordNotFound
	}
	startedAt := s.now()
	requestHash := hashInput(input)
	auditKey := hashInput(struct {
		RequestHash string
		StartedAt   int64
	}{requestHash, startedAt.UnixNano()})
	evidence := pubMedEvidence{QuerySHA256: hashInput(input.Query), Page: input.Page, PageSize: input.PageSize}
	operation := newOperation("query_pubmed", "connector_version", connector.VersionIdentity, "", "", auditKey, requestHash, startedAt)
	operation.IdempotencyKey = ""
	operation.CallerService = "fabric"
	operation.Provider = "ncbi"
	operation.ProviderRequestID = providerRequestID("pubmed", operation.RequestHash)
	if err := s.recordOperation(ctx, operation, "started", evidence, nil); err != nil {
		return PubMedResult{}, err
	}

	result, pmids, err := s.pubmed.query(ctx, input)
	evidence.ResultCount = len(result.Articles)
	evidence.PMIDs = pmids
	if err != nil {
		_ = s.recordOperation(ctx, operation, "failed", evidence, err)
		return PubMedResult{}, err
	}
	if err := s.recordOperation(ctx, operation, "succeeded", evidence, nil); err != nil {
		return PubMedResult{}, err
	}
	return result, nil
}

func (c *pubMedClient) query(ctx context.Context, input PubMedQuery) (PubMedResult, []string, error) {
	searchValues := url.Values{"db": {"pubmed"}, "retmode": {"json"}, "term": {input.Query}, "retstart": {strconv.Itoa((input.Page - 1) * input.PageSize)}, "retmax": {strconv.Itoa(input.PageSize)}}
	body, err := c.get(ctx, "/esearch.fcgi", searchValues)
	if err != nil {
		return PubMedResult{}, nil, err
	}
	var search struct {
		ESearchResult struct {
			Count string   `json:"count"`
			IDs   []string `json:"idlist"`
		} `json:"esearchresult"`
	}
	if err := json.Unmarshal(body, &search); err != nil {
		return PubMedResult{}, nil, fmt.Errorf("pubmed_search_decode: %w", err)
	}
	total, err := strconv.Atoi(search.ESearchResult.Count)
	if err != nil {
		return PubMedResult{}, nil, fmt.Errorf("pubmed_search_count: %w", err)
	}
	result := PubMedResult{Page: input.Page, PageSize: input.PageSize, Total: total, Articles: []PubMedArticle{}}
	if len(search.ESearchResult.IDs) == 0 {
		return result, nil, nil
	}
	fetchValues := url.Values{"db": {"pubmed"}, "retmode": {"xml"}, "id": {strings.Join(search.ESearchResult.IDs, ",")}}
	body, err = c.get(ctx, "/efetch.fcgi", fetchValues)
	if err != nil {
		return PubMedResult{}, search.ESearchResult.IDs, err
	}
	articles, err := decodePubMedArticles(body)
	if err != nil {
		return PubMedResult{}, search.ESearchResult.IDs, err
	}
	result.Articles = articles
	return result, search.ESearchResult.IDs, nil
}

func (c *pubMedClient) get(ctx context.Context, path string, values url.Values) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, c.baseURL+path+"?"+values.Encode(), nil)
		if err != nil {
			cancel()
			return nil, err
		}
		response, err := c.client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
			_ = response.Body.Close()
			cancel()
			if readErr != nil {
				return nil, readErr
			}
			if len(body) > 4<<20 {
				return nil, errors.New("pubmed_response_too_large")
			}
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				return body, nil
			}
			lastErr = fmt.Errorf("pubmed_http_status_%d", response.StatusCode)
			if response.StatusCode != http.StatusTooManyRequests && response.StatusCode < 500 {
				return nil, lastErr
			}
			if attempt < 2 {
				c.sleep(boundedRetryAfter(response.Header.Get("Retry-After")))
			}
			continue
		}
		cancel()
		lastErr = err
		if attempt < 2 {
			c.sleep(100 * time.Millisecond)
		}
	}
	return nil, lastErr
}

func boundedRetryAfter(value string) time.Duration {
	delay, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		when, parseErr := http.ParseTime(strings.TrimSpace(value))
		if parseErr != nil {
			return 100 * time.Millisecond
		}
		until := time.Until(when)
		if until <= 0 {
			return 0
		}
		if until > 2*time.Second {
			return 2 * time.Second
		}
		return until
	}
	if delay < 0 {
		return 100 * time.Millisecond
	}
	if delay > 2 {
		delay = 2
	}
	return time.Duration(delay) * time.Second
}

type textValue string

func (value *textValue) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var text strings.Builder
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			text.Write(current)
		}
	}
	*value = textValue(strings.Join(strings.Fields(text.String()), " "))
	return nil
}

func decodePubMedArticles(body []byte) ([]PubMedArticle, error) {
	var payload struct {
		Articles []struct {
			Citation struct {
				PMID    string `xml:"PMID"`
				Article struct {
					Title   textValue `xml:"ArticleTitle"`
					Authors []struct {
						LastName       string `xml:"LastName"`
						Initials       string `xml:"Initials"`
						CollectiveName string `xml:"CollectiveName"`
					} `xml:"AuthorList>Author"`
					Journal struct {
						Title   string `xml:"Title"`
						PubDate struct {
							Year        string `xml:"Year"`
							MedlineDate string `xml:"MedlineDate"`
						} `xml:"JournalIssue>PubDate"`
					} `xml:"Journal"`
				} `xml:"Article"`
			} `xml:"MedlineCitation"`
			IDs []struct {
				Type  string `xml:"IdType,attr"`
				Value string `xml:",chardata"`
			} `xml:"PubmedData>ArticleIdList>ArticleId"`
		} `xml:"PubmedArticle"`
	}
	if err := xml.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("pubmed_fetch_decode: %w", err)
	}
	articles := make([]PubMedArticle, 0, len(payload.Articles))
	for _, row := range payload.Articles {
		authors := make([]string, 0, len(row.Citation.Article.Authors))
		for _, author := range row.Citation.Article.Authors {
			name := strings.TrimSpace(strings.TrimSpace(author.LastName + " " + author.Initials))
			if name == "" {
				name = strings.TrimSpace(author.CollectiveName)
			}
			if name != "" {
				authors = append(authors, name)
			}
		}
		year := strings.TrimSpace(row.Citation.Article.Journal.PubDate.Year)
		if year == "" && len(row.Citation.Article.Journal.PubDate.MedlineDate) >= 4 {
			year = row.Citation.Article.Journal.PubDate.MedlineDate[:4]
		}
		doi := ""
		for _, id := range row.IDs {
			if strings.EqualFold(id.Type, "doi") {
				doi = strings.TrimSpace(id.Value)
				break
			}
		}
		pmid := strings.TrimSpace(row.Citation.PMID)
		articles = append(articles, PubMedArticle{PMID: pmid, Title: string(row.Citation.Article.Title), Authors: authors, Journal: strings.TrimSpace(row.Citation.Article.Journal.Title), Year: year, DOI: doi, URL: "https://pubmed.ncbi.nlm.nih.gov/" + pmid + "/"})
	}
	return articles, nil
}
