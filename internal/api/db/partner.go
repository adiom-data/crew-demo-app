package db

import (
	"context"
	"database/sql"
	"time"
)

// Partner mirrors a row of the partners table. Enum-like columns (tier, status,
// billing_status) are stored as lowercase text and mapped to proto enums by the
// service layer, keeping this package free of any proto dependency.
type Partner struct {
	ID            string
	Name          string
	ContactEmail  string
	Company       string
	Region        string
	Tier          string
	Status        string
	BillingStatus string
	Notes         string
	CreatedAt     time.Time
}

// Activity mirrors a row of the activities table.
type Activity struct {
	ID        string
	PartnerID string
	Type      string
	Message   string
	CreatedAt time.Time
}

const partnerColumns = `id, name, contact_email, company, region, tier, status, billing_status, notes, created_at`

func scanPartner(row interface{ Scan(...any) error }) (Partner, error) {
	var p Partner
	err := row.Scan(
		&p.ID, &p.Name, &p.ContactEmail, &p.Company, &p.Region,
		&p.Tier, &p.Status, &p.BillingStatus, &p.Notes, &p.CreatedAt,
	)
	return p, err
}

// ListPartners returns every partner, newest first.
func ListPartners(ctx context.Context, db *sql.DB) ([]Partner, error) {
	rows, err := db.QueryContext(ctx, `select `+partnerColumns+` from partners order by created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []Partner
	for rows.Next() {
		p, err := scanPartner(rows)
		if err != nil {
			return nil, err
		}
		partners = append(partners, p)
	}
	return partners, rows.Err()
}

// CountByStatus returns the number of partners keyed by their status text.
func CountByStatus(ctx context.Context, db *sql.DB) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `select status, count(*) from partners group by status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

// GetPartner returns a single partner by id. Returns sql.ErrNoRows if absent.
func GetPartner(ctx context.Context, db *sql.DB, id string) (Partner, error) {
	row := db.QueryRowContext(ctx, `select `+partnerColumns+` from partners where id = $1`, id)
	return scanPartner(row)
}

// GetActivities returns a partner's activity log, newest first.
func GetActivities(ctx context.Context, db *sql.DB, partnerID string) ([]Activity, error) {
	rows, err := db.QueryContext(ctx, `
select id, partner_id, type, message, created_at
from activities
where partner_id = $1
order by created_at desc
`, partnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.PartnerID, &a.Type, &a.Message, &a.CreatedAt); err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, rows.Err()
}

// CreatePartner inserts a partner and returns the stored row. Empty tier, status,
// or billing_status fall back to the table defaults.
func CreatePartner(ctx context.Context, db *sql.DB, p Partner) (Partner, error) {
	row := db.QueryRowContext(ctx, `
insert into partners (name, contact_email, company, region, tier, status, billing_status, notes)
values ($1, $2, $3, $4, coalesce(nullif($5, ''), 'starter'), coalesce(nullif($6, ''), 'pending'), coalesce(nullif($7, ''), 'current'), $8)
returning `+partnerColumns+`
`, p.Name, p.ContactEmail, p.Company, p.Region, p.Tier, p.Status, p.BillingStatus, p.Notes)
	return scanPartner(row)
}

// UpdatePartnerStatus updates a partner's status and returns the stored row.
func UpdatePartnerStatus(ctx context.Context, db *sql.DB, id, status string) (Partner, error) {
	row := db.QueryRowContext(ctx, `
update partners set status = $2, updated_at = now()
where id = $1
returning `+partnerColumns+`
`, id, status)
	return scanPartner(row)
}

// InsertActivity appends an activity to a partner's log.
func InsertActivity(ctx context.Context, db *sql.DB, partnerID, activityType, message string) (Activity, error) {
	var a Activity
	err := db.QueryRowContext(ctx, `
insert into activities (partner_id, type, message)
values ($1, $2, $3)
returning id, partner_id, type, message, created_at
`, partnerID, activityType, message).Scan(&a.ID, &a.PartnerID, &a.Type, &a.Message, &a.CreatedAt)
	return a, err
}
