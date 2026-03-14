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

-- name: UpdateRadusergroupsGroupname :exec
UPDATE radusergroup
SET groupname = sqlc.arg(new_groupname)
WHERE groupname = sqlc.arg(old_groupname);

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

-- name: DeleteRadgrouprepliesByGroupname :exec
DELETE FROM radgroupreply
WHERE groupname = sqlc.arg(groupname);

-- name: InsertRadgroupreply :exec
INSERT INTO radgroupreply (
  groupname,
  attribute,
  op,
  value
) VALUES (
  sqlc.arg(groupname),
  sqlc.arg(attribute),
  sqlc.arg(op),
  sqlc.arg(value)
);
