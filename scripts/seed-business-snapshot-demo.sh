#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
DB_PATH="${DB_PATH:-$ROOT_DIR/data-dev/one-api.db}"

if [[ ! -f "$DB_PATH" ]]; then
  echo "Database not found: $DB_PATH" >&2
  exit 1
fi

sqlite3 "$DB_PATH" <<'SQL'
BEGIN TRANSACTION;

DELETE FROM logs WHERE username LIKE 'snapshot_demo_%';
DELETE FROM top_ups WHERE trade_no LIKE 'snapshot-demo-%';
DELETE FROM users WHERE username LIKE 'snapshot_demo_%';

INSERT INTO users (
  id, username, password, role, status, quota, used_quota, request_count,
  "group", aff_code, aff_count, aff_quota, aff_history, inviter_id,
  created_at, last_login_at
)
VALUES
  (2001, 'snapshot_demo_1', 'demo', 1, 1, 6000000, 0, 0, 'default', 'snapdemo2001', 0, 0, 0, 0, strftime('%s','2026-05-03 09:00:00'), strftime('%s','2026-05-10 18:00:00')),
  (2002, 'snapshot_demo_2', 'demo', 1, 1, 12000000, 0, 0, 'default', 'snapdemo2002', 0, 0, 0, 0, strftime('%s','2026-05-04 10:00:00'), strftime('%s','2026-05-10 18:30:00')),
  (2003, 'snapshot_demo_3', 'demo', 1, 1, 0, 0, 0, 'default', 'snapdemo2003', 0, 0, 0, 0, strftime('%s','2026-05-05 11:00:00'), strftime('%s','2026-05-10 19:00:00')),
  (2004, 'snapshot_demo_4', 'demo', 1, 1, 3500000, 0, 0, 'default', 'snapdemo2004', 0, 0, 0, 0, strftime('%s','2026-05-06 12:00:00'), strftime('%s','2026-05-10 19:30:00')),
  (2005, 'snapshot_demo_5', 'demo', 1, 1, 2500000, 0, 0, 'default', 'snapdemo2005', 0, 0, 0, 0, strftime('%s','2026-05-07 13:00:00'), strftime('%s','2026-05-10 20:00:00')),
  (2006, 'snapshot_demo_6', 'demo', 1, 1, 0, 0, 0, 'default', 'snapdemo2006', 0, 0, 0, 0, strftime('%s','2026-05-08 14:00:00'), strftime('%s','2026-05-10 20:30:00')),
  (2007, 'snapshot_demo_7', 'demo', 1, 1, 8000000, 0, 0, 'default', 'snapdemo2007', 0, 0, 0, 0, strftime('%s','2026-05-09 15:00:00'), strftime('%s','2026-05-10 21:00:00'));

INSERT INTO top_ups (
  id, user_id, amount, money, trade_no, payment_method, create_time, complete_time, status, payment_provider
)
VALUES
  (3001, 2001, 10, 10.0, 'snapshot-demo-topup-1', 'stripe', strftime('%s','2026-05-03 09:30:00'), strftime('%s','2026-05-03 09:35:00'), 'success', 'stripe'),
  (3002, 2002, 20, 20.0, 'snapshot-demo-topup-2', 'stripe', strftime('%s','2026-05-04 10:30:00'), strftime('%s','2026-05-04 10:35:00'), 'success', 'stripe'),
  (3003, 2003, 15, 15.0, 'snapshot-demo-topup-3', 'stripe', strftime('%s','2026-05-05 11:30:00'), strftime('%s','2026-05-05 11:35:00'), 'success', 'stripe'),
  (3004, 2004, 8, 8.0, 'snapshot-demo-topup-4', 'stripe', strftime('%s','2026-05-06 12:30:00'), strftime('%s','2026-05-06 12:35:00'), 'success', 'stripe'),
  (3005, 2007, 30, 30.0, 'snapshot-demo-topup-5', 'stripe', strftime('%s','2026-05-09 15:30:00'), strftime('%s','2026-05-09 15:35:00'), 'success', 'stripe');

INSERT INTO logs (
  id, user_id, created_at, type, content, username, token_name, model_name, quota,
  prompt_tokens, completion_tokens, use_time, is_stream, channel_id, token_id,
  "group", ip, request_id, other
)
VALUES
  (4001, 2001, strftime('%s','2026-05-03 20:00:00'), 2, 'snapshot demo consume', 'snapshot_demo_1', '', 'gpt-4o-mini', 500000, 1200, 800, 4, 0, 1, 0, 'default', '', 'snap-req-1', ''),
  (4002, 2002, strftime('%s','2026-05-04 20:10:00'), 2, 'snapshot demo consume', 'snapshot_demo_2', '', 'gpt-4o-mini', 750000, 1500, 900, 5, 0, 1, 0, 'default', '', 'snap-req-2', ''),
  (4003, 2001, strftime('%s','2026-05-05 20:20:00'), 2, 'snapshot demo consume', 'snapshot_demo_1', '', 'gpt-4o-mini', 350000, 900, 600, 3, 0, 1, 0, 'default', '', 'snap-req-3', ''),
  (4004, 2003, strftime('%s','2026-05-05 20:40:00'), 2, 'snapshot demo consume', 'snapshot_demo_3', '', 'gpt-4o-mini', 250000, 700, 400, 3, 0, 1, 0, 'default', '', 'snap-req-4', ''),
  (4005, 2004, strftime('%s','2026-05-06 21:00:00'), 2, 'snapshot demo consume', 'snapshot_demo_4', '', 'gpt-4o-mini', 450000, 1100, 700, 4, 0, 1, 0, 'default', '', 'snap-req-5', ''),
  (4006, 2005, strftime('%s','2026-05-07 21:10:00'), 2, 'snapshot demo consume', 'snapshot_demo_5', '', 'gpt-4o-mini', 150000, 400, 260, 2, 0, 1, 0, 'default', '', 'snap-req-6', ''),
  (4007, 2006, strftime('%s','2026-05-08 21:20:00'), 2, 'snapshot demo consume', 'snapshot_demo_6', '', 'gpt-4o-mini', 300000, 800, 500, 3, 0, 1, 0, 'default', '', 'snap-req-7', ''),
  (4008, 2002, strftime('%s','2026-05-09 21:30:00'), 2, 'snapshot demo consume', 'snapshot_demo_2', '', 'gpt-4o-mini', 500000, 1000, 700, 4, 0, 1, 0, 'default', '', 'snap-req-8', ''),
  (4009, 2007, strftime('%s','2026-05-09 22:00:00'), 2, 'snapshot demo consume', 'snapshot_demo_7', '', 'gpt-4o-mini', 1000000, 2000, 1300, 6, 0, 1, 0, 'default', '', 'snap-req-9', ''),
  (4010, 2004, strftime('%s','2026-05-10 22:10:00'), 2, 'snapshot demo consume', 'snapshot_demo_4', '', 'gpt-4o-mini', 300000, 700, 420, 3, 0, 1, 0, 'default', '', 'snap-req-10', ''),
  (4011, 2005, strftime('%s','2026-05-10 22:20:00'), 2, 'snapshot demo consume', 'snapshot_demo_5', '', 'gpt-4o-mini', 400000, 850, 560, 3, 0, 1, 0, 'default', '', 'snap-req-11', ''),
  (4012, 2007, strftime('%s','2026-05-10 22:40:00'), 2, 'snapshot demo consume', 'snapshot_demo_7', '', 'gpt-4o-mini', 650000, 1400, 900, 5, 0, 1, 0, 'default', '', 'snap-req-12', '');

COMMIT;
SQL

echo "Seeded business snapshot demo data into: $DB_PATH"
