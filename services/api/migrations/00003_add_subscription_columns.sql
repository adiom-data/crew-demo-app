-- +goose Up
-- Subscription state lives in its own columns rather than reusing billing_status:
-- that enum has no "not subscribed" value and defaults to 'current'. An empty
-- string here means the partner has never subscribed.
alter table partners
  add column if not exists stripe_customer_id     text not null default '',
  add column if not exists stripe_subscription_id text not null default '',
  add column if not exists subscription_plan      text not null default '',
  add column if not exists subscription_status    text not null default '';

-- Lets customer.subscription.* webhooks resolve back to a partner.
create index if not exists partners_stripe_subscription_idx
  on partners (stripe_subscription_id) where stripe_subscription_id <> '';

-- +goose Down
drop index if exists partners_stripe_subscription_idx;
alter table partners
  drop column if exists subscription_status,
  drop column if exists subscription_plan,
  drop column if exists stripe_subscription_id,
  drop column if exists stripe_customer_id;
