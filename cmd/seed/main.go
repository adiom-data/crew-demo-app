// Command seed populates the partners table with realistic demo data so the
// dashboard looks alive. It is idempotent: if any partners already exist it does
// nothing. Configure the database with the standard PG* environment variables.
package main

import (
	"context"
	"log/slog"
	"os"

	apidb "github.com/adiom-data/crew-demo-app/internal/api/db"
	"github.com/caarlos0/env/v11"
)

type environment struct {
	PGHost     string `env:"PGHOST" envDefault:"localhost"`
	PGPort     string `env:"PGPORT" envDefault:"5432"`
	PGDatabase string `env:"PGDATABASE" envDefault:"app"`
	PGUser     string `env:"PGUSER" envDefault:"postgres"`
	PGPassword string `env:"PGPASSWORD"`
	PGSSLMode  string `env:"PGSSLMODE" envDefault:"disable"`
}

type seedPartner struct {
	name     string
	email    string
	company  string
	region   string
	tier     string
	status   string
	billing  string
	notes    string
	activity string // optional extra activity message beyond the "created" entry
}

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	var e environment
	if err := env.Parse(&e); err != nil {
		return err
	}

	db, err := apidb.Open(apidb.Config{
		Host:     e.PGHost,
		Port:     e.PGPort,
		Database: e.PGDatabase,
		User:     e.PGUser,
		Password: e.PGPassword,
		SSLMode:  e.PGSSLMode,
	})
	if err != nil {
		return err
	}
	defer db.Close()

	ctx := context.Background()
	if err := apidb.Ping(ctx, db); err != nil {
		return err
	}

	existing, err := apidb.ListPartners(ctx, db)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		slog.Info("partners already present; skipping seed", "count", len(existing))
		return nil
	}

	inserted := 0
	for _, sp := range seedPartners() {
		partner, err := apidb.CreatePartner(ctx, db, apidb.Partner{
			Name:          sp.name,
			ContactEmail:  sp.email,
			Company:       sp.company,
			Region:        sp.region,
			Tier:          sp.tier,
			Status:        sp.status,
			BillingStatus: sp.billing,
			Notes:         sp.notes,
		})
		if err != nil {
			return err
		}
		if _, err := apidb.InsertActivity(ctx, db, partner.ID, "created", "Partner onboarded"); err != nil {
			return err
		}
		if sp.activity != "" {
			if _, err := apidb.InsertActivity(ctx, db, partner.ID, "note", sp.activity); err != nil {
				return err
			}
		}
		inserted++
	}

	slog.Info("seed complete", "partners", inserted)
	return nil
}

func seedPartners() []seedPartner {
	return []seedPartner{
		{"Amara Okafor", "amara@brightpath.io", "BrightPath Health", "US-East", "enterprise", "active", "current", "Top-tier account; QBR scheduled.", "Renewed annual contract"},
		{"Liam Chen", "liam@northwind.co", "Northwind Analytics", "US-West", "pro", "active", "current", "", "Expanded to 3 seats"},
		{"Sofia Martins", "sofia@vela.com", "Vela Logistics", "EU-West", "pro", "active", "past_due", "Invoice reminder sent.", ""},
		{"Noah Williams", "noah@pinehill.org", "Pinehill Clinics", "US-Central", "starter", "pending", "trialing", "Awaiting signed BAA.", ""},
		{"Yuki Tanaka", "yuki@sakura.jp", "Sakura Retail", "APAC", "enterprise", "active", "current", "", "Launched marketplace integration"},
		{"Emma Johansson", "emma@fjord.no", "Fjord Payments", "EU-North", "pro", "active", "current", "", ""},
		{"Diego Ramirez", "diego@andes.cl", "Andes Freight", "LATAM", "starter", "churned", "current", "Churned: budget cuts.", "Marked churned"},
		{"Priya Nair", "priya@lotus.in", "Lotus EdTech", "APAC", "pro", "pending", "trialing", "Pilot in progress.", ""},
		{"Oliver Smith", "oliver@harborline.uk", "Harborline Marine", "EU-West", "starter", "active", "current", "", ""},
		{"Fatima Al-Sayed", "fatima@zenith.ae", "Zenith Wealth", "MEA", "enterprise", "active", "current", "Strategic logo.", ""},
		{"Lucas Rossi", "lucas@ferro.it", "Ferro Manufacturing", "EU-South", "pro", "active", "past_due", "", ""},
		{"Grace Kim", "grace@meadow.kr", "Meadow Beauty", "APAC", "starter", "pending", "trialing", "", ""},
		{"Ethan Brown", "ethan@summit.ca", "Summit Outdoors", "US-West", "pro", "active", "current", "", "Upgraded to Pro"},
		{"Isabella Ferreira", "isabella@brasa.br", "Brasa Foods", "LATAM", "enterprise", "active", "current", "", ""},
		{"Mateo Garcia", "mateo@costa.es", "Costa Travel", "EU-South", "starter", "churned", "current", "Seasonal churn.", ""},
		{"Hannah Weber", "hannah@alpen.de", "Alpen Insurance", "EU-Central", "pro", "active", "current", "", ""},
		{"Kofi Mensah", "kofi@baobab.gh", "Baobab Agritech", "MEA", "starter", "pending", "trialing", "Field trial ongoing.", ""},
		{"Mia Andersen", "mia@nordlys.dk", "Nordlys Energy", "EU-North", "enterprise", "active", "current", "", "Signed multi-year deal"},
		{"Aarav Patel", "aarav@indus.in", "Indus Mobility", "APAC", "pro", "active", "past_due", "Follow up on billing.", ""},
		{"Chloe Dubois", "chloe@lumiere.fr", "Lumiere Media", "EU-West", "starter", "active", "current", "", ""},
		{"Benjamin Lee", "ben@cedar.sg", "Cedar Fintech", "APAC", "enterprise", "active", "current", "", ""},
		{"Zara Khan", "zara@meridian.pk", "Meridian Textiles", "MEA", "pro", "pending", "trialing", "", ""},
		{"Daniel Novak", "daniel@vltava.cz", "Vltava Software", "EU-Central", "starter", "active", "current", "", ""},
		{"Ava Thompson", "ava@coral.au", "Coral Reef Tours", "APAC", "starter", "churned", "current", "Churned after trial.", ""},
		{"Omar Haddad", "omar@atlas.ma", "Atlas Logistics", "MEA", "pro", "active", "current", "", "Added warehouse module"},
		{"Line Larsen", "line@bjork.se", "Bjork Retail", "EU-North", "enterprise", "active", "current", "", ""},
		{"Ryan Walsh", "ryan@shamrock.ie", "Shamrock Labs", "EU-West", "pro", "pending", "trialing", "Security review pending.", ""},
		{"Nadia Petrova", "nadia@volga.bg", "Volga Analytics", "EU-East", "starter", "active", "current", "", ""},
		{"Carlos Mendoza", "carlos@aztec.mx", "Aztec Payments", "LATAM", "pro", "active", "past_due", "", ""},
		{"Julia Costa", "julia@tejo.pt", "Tejo Health", "EU-South", "enterprise", "active", "current", "Reference customer.", ""},
	}
}
