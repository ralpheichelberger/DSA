package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Keep in-memory tests pinned to a single connection.
	if dbPath == ":memory:" {
		db.SetMaxOpenConns(1)
	}

	s := &Store{db: db}
	if err := s.Migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS product_tests (
			id TEXT PRIMARY KEY,
			product_name TEXT NOT NULL,
			product_image_url TEXT NOT NULL DEFAULT '',
			niche TEXT NOT NULL,
			shopify_store TEXT NOT NULL,
			source_platform TEXT NOT NULL,
			supplier TEXT NOT NULL,
			cogs_eur REAL NOT NULL,
			sell_price_eur REAL NOT NULL,
			gross_margin_pct REAL NOT NULL,
			beroas REAL NOT NULL,
			shipping_cost_eur REAL NOT NULL,
			shipping_days INTEGER NOT NULL,
			status TEXT NOT NULL,
			kill_reason TEXT NOT NULL,
			score INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS campaign_results (
			id TEXT PRIMARY KEY,
			product_test_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			campaign_id TEXT NOT NULL,
			spend_eur REAL NOT NULL,
			revenue_eur REAL NOT NULL,
			roas REAL NOT NULL,
			ctr_pct REAL NOT NULL,
			cpa_eur REAL NOT NULL,
			impressions INTEGER NOT NULL,
			clicks INTEGER NOT NULL,
			purchases INTEGER NOT NULL,
			days_running INTEGER NOT NULL,
			snapshot_date DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			FOREIGN KEY(product_test_id) REFERENCES product_tests(id)
		);`,
		`CREATE TABLE IF NOT EXISTS learned_lessons (
			id TEXT PRIMARY KEY,
			category TEXT NOT NULL,
			lesson TEXT NOT NULL,
			confidence REAL NOT NULL,
			evidence_count INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS creative_performance (
			id TEXT PRIMARY KEY,
			product_test_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			creative_type TEXT NOT NULL,
			hook_description TEXT NOT NULL,
			angle TEXT NOT NULL,
			ctr_pct REAL NOT NULL,
			hook_retention_3s_pct REAL NOT NULL,
			spend_eur REAL NOT NULL,
			roas REAL NOT NULL,
			won BOOLEAN NOT NULL,
			created_at DATETIME NOT NULL,
			FOREIGN KEY(product_test_id) REFERENCES product_tests(id)
		);`,
	}

	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`ALTER TABLE product_tests ADD COLUMN product_image_url TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return err
		}
	}
	for _, col := range []string{"ad_url", "shop_url", "landing_url"} {
		if _, err := s.db.Exec(fmt.Sprintf(`ALTER TABLE product_tests ADD COLUMN %s TEXT NOT NULL DEFAULT ''`, col)); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
				return err
			}
		}
	}

	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SaveProductTest(pt ProductTest) error {
	_, err := s.db.Exec(
		`INSERT INTO product_tests (
			id, product_name, product_image_url, ad_url, shop_url, landing_url, niche, shopify_store, source_platform, supplier, cogs_eur, sell_price_eur,
			gross_margin_pct, beroas, shipping_cost_eur, shipping_days, status, kill_reason, score, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			product_name = excluded.product_name,
			product_image_url = excluded.product_image_url,
			ad_url = excluded.ad_url,
			shop_url = excluded.shop_url,
			landing_url = excluded.landing_url,
			niche = excluded.niche,
			shopify_store = excluded.shopify_store,
			source_platform = excluded.source_platform,
			supplier = excluded.supplier,
			cogs_eur = excluded.cogs_eur,
			sell_price_eur = excluded.sell_price_eur,
			gross_margin_pct = excluded.gross_margin_pct,
			beroas = excluded.beroas,
			shipping_cost_eur = excluded.shipping_cost_eur,
			shipping_days = excluded.shipping_days,
			status = excluded.status,
			kill_reason = excluded.kill_reason,
			score = excluded.score,
			updated_at = excluded.updated_at`,
		pt.ID, pt.ProductName, pt.ProductImageURL, pt.AdURL, pt.ShopURL, pt.LandingURL, pt.Niche, pt.ShopifyStore, pt.SourcePlatform, pt.Supplier, pt.COGSEur, pt.SellPriceEur,
		pt.GrossMarginPct, pt.BEROAS, pt.ShippingCostEur, pt.ShippingDays, pt.Status, pt.KillReason, pt.Score, pt.CreatedAt, pt.UpdatedAt,
	)
	return err
}

func (s *Store) UpdateProductStatus(id string, status string, killReason string) error {
	_, err := s.db.Exec(
		`UPDATE product_tests SET status = ?, kill_reason = ?, updated_at = ? WHERE id = ?`,
		status, killReason, time.Now().UTC(), id,
	)
	return err
}

func (s *Store) GetProductTest(id string) (*ProductTest, error) {
	row := s.db.QueryRow(
		`SELECT id, product_name, product_image_url, ad_url, shop_url, landing_url, niche, shopify_store, source_platform, supplier, cogs_eur, sell_price_eur,
		        gross_margin_pct, beroas, shipping_cost_eur, shipping_days, status, kill_reason, score, created_at, updated_at
		   FROM product_tests WHERE id = ?`,
		id,
	)

	pt := ProductTest{}
	err := row.Scan(
		&pt.ID, &pt.ProductName, &pt.ProductImageURL, &pt.AdURL, &pt.ShopURL, &pt.LandingURL, &pt.Niche, &pt.ShopifyStore, &pt.SourcePlatform, &pt.Supplier, &pt.COGSEur, &pt.SellPriceEur,
		&pt.GrossMarginPct, &pt.BEROAS, &pt.ShippingCostEur, &pt.ShippingDays, &pt.Status, &pt.KillReason, &pt.Score, &pt.CreatedAt, &pt.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pt, nil
}

func (s *Store) GetProductsByStatus(status string) ([]ProductTest, error) {
	return s.getProducts(`SELECT id, product_name, product_image_url, ad_url, shop_url, landing_url, niche, shopify_store, source_platform, supplier, cogs_eur, sell_price_eur,
		gross_margin_pct, beroas, shipping_cost_eur, shipping_days, status, kill_reason, score, created_at, updated_at
		FROM product_tests WHERE status = ? ORDER BY updated_at DESC`, status)
}

func (s *Store) GetProductsByNiche(niche string) ([]ProductTest, error) {
	return s.getProducts(`SELECT id, product_name, product_image_url, ad_url, shop_url, landing_url, niche, shopify_store, source_platform, supplier, cogs_eur, sell_price_eur,
		gross_margin_pct, beroas, shipping_cost_eur, shipping_days, status, kill_reason, score, created_at, updated_at
		FROM product_tests WHERE niche = ? ORDER BY updated_at DESC`, niche)
}

func (s *Store) GetAllProducts() ([]ProductTest, error) {
	return s.getProducts(`SELECT id, product_name, product_image_url, ad_url, shop_url, landing_url, niche, shopify_store, source_platform, supplier, cogs_eur, sell_price_eur,
		gross_margin_pct, beroas, shipping_cost_eur, shipping_days, status, kill_reason, score, created_at, updated_at
		FROM product_tests ORDER BY updated_at DESC`)
}

func (s *Store) SaveCampaignResult(cr CampaignResult) error {
	_, err := s.db.Exec(
		`INSERT INTO campaign_results (
			id, product_test_id, platform, campaign_id, spend_eur, revenue_eur, roas, ctr_pct, cpa_eur,
			impressions, clicks, purchases, days_running, snapshot_date, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cr.ID, cr.ProductTestID, cr.Platform, cr.CampaignID, cr.SpendEur, cr.RevenueEur, cr.ROAS, cr.CTRPct, cr.CPAEur,
		cr.Impressions, cr.Clicks, cr.Purchases, cr.DaysRunning, cr.SnapshotDate, cr.CreatedAt,
	)
	return err
}

func (s *Store) GetCampaignResultsForProduct(productTestID string) ([]CampaignResult, error) {
	rows, err := s.db.Query(
		`SELECT id, product_test_id, platform, campaign_id, spend_eur, revenue_eur, roas, ctr_pct, cpa_eur,
		        impressions, clicks, purchases, days_running, snapshot_date, created_at
		   FROM campaign_results WHERE product_test_id = ? ORDER BY snapshot_date DESC`,
		productTestID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CampaignResult
	for rows.Next() {
		var cr CampaignResult
		if err := rows.Scan(
			&cr.ID, &cr.ProductTestID, &cr.Platform, &cr.CampaignID, &cr.SpendEur, &cr.RevenueEur, &cr.ROAS, &cr.CTRPct, &cr.CPAEur,
			&cr.Impressions, &cr.Clicks, &cr.Purchases, &cr.DaysRunning, &cr.SnapshotDate, &cr.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, cr)
	}
	return results, rows.Err()
}

func (s *Store) GetActiveCampaigns() ([]CampaignResult, error) {
	rows, err := s.db.Query(
		`SELECT id, product_test_id, platform, campaign_id, spend_eur, revenue_eur, roas, ctr_pct, cpa_eur,
		        impressions, clicks, purchases, days_running, snapshot_date, created_at
		   FROM campaign_results WHERE days_running > 0 ORDER BY snapshot_date DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CampaignResult
	for rows.Next() {
		var cr CampaignResult
		if err := rows.Scan(
			&cr.ID, &cr.ProductTestID, &cr.Platform, &cr.CampaignID, &cr.SpendEur, &cr.RevenueEur, &cr.ROAS, &cr.CTRPct, &cr.CPAEur,
			&cr.Impressions, &cr.Clicks, &cr.Purchases, &cr.DaysRunning, &cr.SnapshotDate, &cr.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, cr)
	}
	return results, rows.Err()
}

func (s *Store) SaveLearnedLesson(ll LearnedLesson) error {
	_, err := s.db.Exec(
		`INSERT INTO learned_lessons (id, category, lesson, confidence, evidence_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ll.ID, ll.Category, ll.Lesson, ll.Confidence, ll.EvidenceCount, ll.CreatedAt, ll.UpdatedAt,
	)
	return err
}

func (s *Store) UpdateLessonConfidence(id string, confidence float64, evidenceCount int) error {
	_, err := s.db.Exec(
		`UPDATE learned_lessons SET confidence = ?, evidence_count = ?, updated_at = ? WHERE id = ?`,
		confidence, evidenceCount, time.Now().UTC(), id,
	)
	return err
}

func (s *Store) GetTopLessons(category string, limit int) ([]LearnedLesson, error) {
	query := `SELECT id, category, lesson, confidence, evidence_count, created_at, updated_at
		FROM learned_lessons`
	var args []any
	if strings.TrimSpace(category) != "" {
		query += ` WHERE category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY confidence DESC, evidence_count DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []LearnedLesson
	for rows.Next() {
		var ll LearnedLesson
		if err := rows.Scan(
			&ll.ID, &ll.Category, &ll.Lesson, &ll.Confidence, &ll.EvidenceCount, &ll.CreatedAt, &ll.UpdatedAt,
		); err != nil {
			return nil, err
		}
		lessons = append(lessons, ll)
	}
	return lessons, rows.Err()
}

func (s *Store) GetAllLessons() ([]LearnedLesson, error) {
	return s.GetTopLessons("", 0)
}

func (s *Store) SaveCreativePerformance(cp CreativePerformance) error {
	_, err := s.db.Exec(
		`INSERT INTO creative_performance (
			id, product_test_id, platform, creative_type, hook_description, angle, ctr_pct, hook_retention_3s_pct,
			spend_eur, roas, won, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cp.ID, cp.ProductTestID, cp.Platform, cp.CreativeType, cp.HookDescription, cp.Angle, cp.CTRPct, cp.HookRetention3sPct,
		cp.SpendEur, cp.ROAS, cp.Won, cp.CreatedAt,
	)
	return err
}

func (s *Store) GetCreativeWinners(platform string) ([]CreativePerformance, error) {
	rows, err := s.db.Query(
		`SELECT id, product_test_id, platform, creative_type, hook_description, angle, ctr_pct, hook_retention_3s_pct,
		        spend_eur, roas, won, created_at
		   FROM creative_performance WHERE won = 1 AND platform = ? ORDER BY created_at DESC`,
		platform,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CreativePerformance
	for rows.Next() {
		var cp CreativePerformance
		if err := rows.Scan(
			&cp.ID, &cp.ProductTestID, &cp.Platform, &cp.CreativeType, &cp.HookDescription, &cp.Angle, &cp.CTRPct, &cp.HookRetention3sPct,
			&cp.SpendEur, &cp.ROAS, &cp.Won, &cp.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, cp)
	}
	return results, rows.Err()
}

func (s *Store) BuildMemoryContext() (string, error) {
	lessons, err := s.GetTopLessons("", 10)
	if err != nil {
		return "", err
	}

	rows, err := s.db.Query(
		`SELECT product_name, niche, status, gross_margin_pct, score
		   FROM product_tests ORDER BY updated_at DESC LIMIT 10`,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var products []ProductTest
	for rows.Next() {
		var p ProductTest
		if err := rows.Scan(&p.ProductName, &p.Niche, &p.Status, &p.GrossMarginPct, &p.Score); err != nil {
			return "", err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("## Lessons from past campaigns:\n")
	for _, l := range lessons {
		b.WriteString(fmt.Sprintf("- [%s] %s (confidence: %.0f%%)\n", l.Category, l.Lesson, l.Confidence*100))
	}
	b.WriteString("\n## Recent product outcomes (last 10):\n")
	for _, p := range products {
		b.WriteString(fmt.Sprintf("- %s | %s | status: %s | margin: %.0f%% | score: %d\n", p.ProductName, p.Niche, p.Status, p.GrossMarginPct, p.Score))
	}

	return b.String(), nil
}

func (s *Store) getProducts(query string, args ...any) ([]ProductTest, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []ProductTest
	for rows.Next() {
		var pt ProductTest
		if err := rows.Scan(
			&pt.ID, &pt.ProductName, &pt.ProductImageURL, &pt.AdURL, &pt.ShopURL, &pt.LandingURL, &pt.Niche, &pt.ShopifyStore, &pt.SourcePlatform, &pt.Supplier, &pt.COGSEur, &pt.SellPriceEur,
			&pt.GrossMarginPct, &pt.BEROAS, &pt.ShippingCostEur, &pt.ShippingDays, &pt.Status, &pt.KillReason, &pt.Score, &pt.CreatedAt, &pt.UpdatedAt,
		); err != nil {
			return nil, err
		}
		products = append(products, pt)
	}
	return products, rows.Err()
}
