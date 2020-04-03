# sqlite3 :memory: <<EOF
# CREATE TABLE IF NOT EXISTS ticks (time INTEGER PRIMARY KEY ASC, labels TEXT);
# CREATE TABLE IF NOT EXISTS watches (last_write INTEGER PRIMARY KEY ASC, dir TEXT, label TEXT);
#
# INSERT INTO watches (last_write, dir, label) VALUES (1, "/test", "label 1");
# SELECT * FROM watches;
# INSERT INTO ticks (time, labels) VALUES (1, "label 1");
#
# BEGIN TRANSACTION;
# INSERT INTO ticks (time, labels) VALUES (2, "label 1");
# UPDATE watches SET last_write = 2 WHERE dir = "/test";
# COMMIT;
#
# BEGIN TRANSACTION;
# INSERT INTO ticks (time, labels) VALUES (3, "label 2");
# UPDATE watches SET last_write = 3 WHERE dir = "/test2";
# COMMIT;
#
# SELECT "--------------------------------";
# SELECT * FROM watches;
# SELECT "--------------------------------";
# SELECT * FROM ticks;
# SELECT "--------------------------------";
# EOF
#
# sqlite3 :memory: <<EOF
# CREATE TABLE IF NOT EXISTS watches (last_write INTEGER PRIMARY KEY ASC, dir TEXT, label TEXT);
#
# INSERT INTO watches (last_write, dir, label) VALUES (1, "/test", "label 1");
# INSERT INTO watches (last_write, dir, label) VALUES (2, "/test2", "label 1");
# INSERT INTO watches (last_write, dir, label) VALUES (3, "/test3", "label 1");
# INSERT INTO watches (last_write, dir, label) VALUES (4, "/test4", "label 1");
# INSERT INTO watches (last_write, dir, label) VALUES (5, "/test5", "label 1");
# SELECT * FROM watches;
#
# DELETE FROM watches
# WHERE dir IN (
#   SELECT dir FROM watches
#   LIMIT (SELECT COUNT(*) FROM watches) - 3);
#
# SELECT "--------------------------------";
# SELECT * FROM watches;
# EOF

sqlite3 :memory: <<EOF
CREATE TABLE IF NOT EXISTS watches (last_write INTEGER PRIMARY KEY ASC, dir TEXT, label TEXT);

INSERT INTO watches (last_write, dir, label) VALUES (1, "/test", "label 1");
INSERT INTO watches (last_write, dir, label) VALUES (2, "/test2", "label 1");
INSERT INTO watches (last_write, dir, label) VALUES (3, "/test3", "label 1");
INSERT INTO watches (last_write, dir, label) VALUES (4, "/test4", "label 1");
INSERT INTO watches (last_write, dir, label) VALUES (5, "/test5", "label 1");
SELECT * FROM watches;
SELECT "--------------------------------";
SELECT COUNT(*) FROM watches;
SELECT ((SELECT COUNT(*) FROM watches) - 8);
SELECT MAX(0, (SELECT COUNT(*) FROM watches) - 8);
SELECT dir FROM watches LIMIT MAX(0, (SELECT COUNT(*) FROM watches) - 8);

SELECT "--------------------------------";
DELETE FROM watches
WHERE dir IN (
  SELECT dir FROM watches
  LIMIT MAX(0, (SELECT COUNT(*) FROM watches) - 8)
);

SELECT * FROM watches;
SELECT "--------------------------------";
EOF
