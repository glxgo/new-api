import json
import unittest

from pull_topology import parse_rows


class TopologyExportTests(unittest.TestCase):
    def rows(self):
        return [
            {'kind': 'channel', 'data': {'id': 8, 'name': 'fixture', 'status': 2, 'group': 'a,b', 'models': 'm'}},
            {'kind': 'ability', 'data': {'channel_id': 8, 'group': 'a', 'model': 'm', 'enabled': 0}},
            {'kind': 'option', 'data': {'key': 'GroupRatio', 'value': '{"a":1}'}},
            {'kind': 'option', 'data': {'key': 'UserUsableGroups', 'value': '{}'}},
        ]

    def parse(self, rows):
        return parse_rows(json.dumps(r) for r in rows)

    def test_disabled_membership_is_preserved(self):
        snapshot = self.parse(self.rows())
        self.assertIs(snapshot['abilities'][0]['enabled'], False)
        self.assertEqual(snapshot['channels'][0]['group'], 'a,b')

    def test_credentials_and_endpoints_rejected(self):
        for field in ['key', 'base_url', 'password']:
            with self.subTest(field=field):
                rows = self.rows()
                rows[0]['data'][field] = 'must-not-export'
                with self.assertRaises(ValueError):
                    self.parse(rows)

    def test_incomplete_or_unapproved_options_rejected(self):
        with self.assertRaises(ValueError):
            self.parse(self.rows()[:-1])
        rows = self.rows()
        rows.append({'kind': 'option', 'data': {'key': 'unapproved', 'value': 'private'}})
        with self.assertRaises(ValueError):
            self.parse(rows)


if __name__ == '__main__':
    unittest.main()
