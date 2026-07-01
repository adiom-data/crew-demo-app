-- +goose Up
create table if not exists partners (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  contact_email text not null,
  company text not null default '',
  region text not null default '',
  tier text not null default 'starter',
  status text not null default 'pending',
  billing_status text not null default 'current',
  notes text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists partners_status_idx on partners (status);
create index if not exists partners_created_at_idx on partners (created_at desc);

create table if not exists activities (
  id uuid primary key default gen_random_uuid(),
  partner_id uuid not null references partners(id) on delete cascade,
  type text not null default '',
  message text not null default '',
  created_at timestamptz not null default now()
);

create index if not exists activities_partner_idx
  on activities (partner_id, created_at desc);

-- +goose Down
drop index if exists activities_partner_idx;
drop table if exists activities;
drop index if exists partners_created_at_idx;
drop index if exists partners_status_idx;
drop table if exists partners;
