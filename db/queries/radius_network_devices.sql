-- name: UpsertRadcheckCleartextPassword :exec
INSERT INTO radcheck (
  username,
  attribute,
  op,
  value
) VALUES (
  sqlc.arg(username),
  'Cleartext-Password',
  ':=',
  sqlc.arg(value)
)
ON CONFLICT (username, attribute) DO UPDATE SET
  op = EXCLUDED.op,
  value = EXCLUDED.value;

-- name: DeleteRadcheckCleartextPasswordByUsername :exec
DELETE FROM radcheck
WHERE username = sqlc.arg(username)
  AND attribute = 'Cleartext-Password';

-- name: DeleteRadusergroupsByUsername :exec
DELETE FROM radusergroup
WHERE username = sqlc.arg(username);

-- name: InsertRadusergroup :exec
INSERT INTO radusergroup (
  username,
  groupname,
  priority
) VALUES (
  sqlc.arg(username),
  sqlc.arg(groupname),
  sqlc.arg(priority)
);
