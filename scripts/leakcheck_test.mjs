#!/usr/bin/env node
// The leak check's rules against synthetic lines. Every bad string is assembled at
// run time, so this file does not trip the check it tests.
import assert from 'node:assert/strict'
import { loadNames, scanLine } from './leakcheck.mjs'

const j = (...p) => p.join('')
const rules = (line, names) => scanLine(line, names).map((h) => h.rule)

const bad = [
  [j('see /', 'Users', '/someone/projects/x'), 'home path'],
  [j('cat /', 'home', '/someone/.ssh/config'), 'home path'],
  [j('open file', '://tmp/x.html'), 'file URL'],
  [j('dial 10', '.1.2.3:8889'), 'private IPv4'],
  [j('host 192', '.168.0.10'), 'private IPv4'],
  [j('bridge 172', '.17.0.1'), 'private IPv4'],
  [j('token gh', 'p_', 'a'.repeat(36)), 'GitHub token'],
  [j('AK', 'IA', 'ABCDEFGHIJKLMNOP'), 'AWS key'],
  [j('-----BEGIN RSA PRIV', 'ATE KEY-----'), 'private key'],
  [j('Co-', 'Authored-By: Clau', 'de <x@y.z>'), 'attribution trailer'],
  [j('Gener', 'ated with [Clau', 'de Code](https://example.com)'), 'attribution footer'],
  [j('mail someone', '@', 'company.io'), 'e-mail address'],
]
for (const [line, want] of bad) assert.ok(rules(line).includes(want), `${want} not caught in ${JSON.stringify(line)}`)

const good = [
  'author Allan Nava <allannava95@gmail.com>',
  'committer GitHub <noreply@github.com>',
  '12345+someone@users.noreply.github.com',
  'POST to https://edge.example.test/whep',
  'listen 127.0.0.1:8889 and [::1]:8889',
  'go 1.27 and golang.org/x/net v0.50.0, actions/checkout@v7, pion/webrtc/v4@v4.2.22',
  '172.32.0.1 is public and 11.0.0.1 is too',
  'absolute home paths are rejected',
]
for (const line of good) assert.deepEqual(rules(line), [], `false positive on ${JSON.stringify(line)}`)

// Denylisted names: case-insensitive, across - _ and . joins, inside compounds.
const names = loadNames({ LEAKCHECK_DENYLIST: '# a comment\nacme-corp\nzorblat\n' })
assert.deepEqual(names, ['acmecorp', 'zorblat'])
for (const line of ['ACME-Corp', 'acme_corp', 'deploy_acmecorp_prod', 'Zorblat-Media', 'host.zorblat.example']) {
  assert.ok(rules(line, names).includes('denylisted name'), `name not caught in ${line}`)
}
for (const line of ['acme corp', 'zorb lat', 'nothing here']) assert.deepEqual(rules(line, names), [], `false positive on ${line}`)
assert.ok(!JSON.stringify(scanLine('zorblat', names)).includes('zorblat'), 'a finding must not echo the denylisted name')

console.log(`ok — leakcheck: ${bad.length} leaks caught, ${good.length} clean lines pass, names matched across joins`)
