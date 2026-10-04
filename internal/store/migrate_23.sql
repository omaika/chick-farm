-- v22 -> v23: the max_rework_across watch rule counts the rework mails a member sent, every tick;
-- without this index that is a scan of all mail. Only reworks are indexed.
CREATE INDEX messages_rework ON messages(from_id, seq) WHERE kind = 'rework';
