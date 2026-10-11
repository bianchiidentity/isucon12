#!/usr/bin/env python3
"""Check compaction partitions, ties, original rows and repeatability."""
import pathlib
import sqlite3
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]

class CompactionTest(unittest.TestCase):
    def test_keeps_only_latest_per_tenant_competition_player(self):
        db = sqlite3.connect(':memory:')
        db.executescript((ROOT / 'webapp/sql/tenant/10_schema.sql').read_text())
        rows = [
            ('a', 1, 'p', 'c', 999, 1, 10, 11),
            ('b', 1, 'p', 'c', -5, 3, 12, 13),
            ('tie', 1, 'p', 'c', 7, 3, 14, 15),
            ('other-player', 1, 'q', 'c', 10, 1, 10, 11),
            ('other-competition', 1, 'p', 'd', 20, 1, 10, 11),
            ('other-tenant', 2, 'p', 'c', 30, 1, 10, 11),
        ]
        db.executemany('INSERT INTO player_score VALUES (?,?,?,?,?,?,?,?)', rows)
        db.commit()
        script = (ROOT / 'webapp/sql/tenant/30_compact_scores.sql').read_text()
        expected = sorted(rows[1:])
        for _ in range(2):
            db.executescript(script)
            self.assertEqual(expected, db.execute('SELECT * FROM player_score ORDER BY id').fetchall())
        db.execute('DELETE FROM player_score')
        db.commit()
        db.executescript(script)
        self.assertEqual(0, db.execute('SELECT count(*) FROM player_score').fetchone()[0])
        db.close()

if __name__ == '__main__':
    unittest.main()
