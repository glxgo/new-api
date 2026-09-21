/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import React, { useState } from 'react';
import { Button, Form, Modal, Toast, Tooltip } from '@douyinfe/semi-ui';
import { Copy, Monitor } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import Codex from '@lobehub/icons/es/Codex';
import ClaudeCode from '@lobehub/icons/es/ClaudeCode';
import OpenCode from '@lobehub/icons/es/OpenCode';
import OpenClaw from '@lobehub/icons/es/OpenClaw';
import CherryStudio from '@lobehub/icons/es/CherryStudio';
import OpenAI from '@lobehub/icons/es/OpenAI';
import { copy } from '../../../helpers';

const icons = {
  codex: Codex,
  claude_code: ClaudeCode,
  opencode: OpenCode,
  openclaw: OpenClaw,
  cherry_studio: CherryStudio,
  openai_sdk: OpenAI,
};
const tints = {
  codex: '113,113,122',
  claude_code: '249,115,22',
  pi: '59,130,246',
  opencode: '168,85,247',
  newapi: '8,145,178',
  zcode: '107,114,128',
  deepseek_harness: '59,130,246',
};
const families = [
  ['codex', 'Codex'],
  ['claude_code', 'Claude Code'],
  ['pi', 'Pi'],
  ['opencode', 'OpenCode'],
  ['newapi', 'NewAPI'],
  ['zcode', 'ZCode'],
  ['deepseek_harness', 'DeepSeek Harness (DSH)'],
  ['openclaw', 'OpenClaw'],
  ['cherry_studio', 'Cherry Studio'],
  ['openai_sdk', 'OpenAI SDK'],
  ['node', 'Node'],
  ['bun', 'Bun'],
  ['python', 'Python'],
  ['browser', 'Browser'],
  ['curl', 'curl'],
  ['unknown', 'Unknown client'],
  ['unrecorded', 'Not recorded'],
];

export function ClientFamilyField() {
  const { t } = useTranslation();
  return (
    <Form.Select
      field='client_family'
      aria-label={t('Client category')}
      placeholder={t('All clients')}
      showClear
      pure
      size='small'
      optionList={families.map(([value, label]) => ({
        value,
        label: t(label),
      }))}
    />
  );
}

export default function ClientTimeCell({ time, status, client, coding }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const Icon = icons[client?.family] || Monitor;
  const tint = tints[client?.family];
  return (
    <div
      style={{
        position: 'relative',
        isolation: 'isolate',
        minWidth: 0,
        padding: 4,
        borderRadius: 4,
        overflow: 'hidden',
      }}
    >
      {tint && (
        <div
          aria-hidden='true'
          style={{
            position: 'absolute',
            zIndex: -1,
            bottom: 0,
            left: 0,
            right: 0,
            height: 20,
            pointerEvents: 'none',
            background: `linear-gradient(to top, rgba(${tint},.15), transparent)`,
          }}
        />
      )}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          gap: 6,
          fontSize: 12,
        }}
      >
        {time}
        {status}
      </div>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          marginTop: 4,
          minHeight: 20,
        }}
      >
        {client ? (
          <Button
            type='tertiary'
            theme='borderless'
            size='small'
            aria-label={`${t('Client details')}: ${t(client.display_name)}`}
            onClick={() => setOpen(true)}
            icon={<Icon size={14} aria-hidden='true' />}
            style={{
              maxWidth: '100%',
              whiteSpace: 'normal',
              height: 'auto',
              textAlign: 'left',
            }}
          >
            {t(client.display_name)}
          </Button>
        ) : (
          <span style={{ fontSize: 12, color: 'var(--semi-color-text-2)' }}>
            {t('Not recorded')}
          </span>
        )}
        {coding && (
          <span
            style={{
              fontSize: 10,
              color: 'var(--semi-color-text-2)',
              flexShrink: 0,
            }}
          >
            Coding
          </span>
        )}
      </div>
      {client && (
        <Modal
          title={t('Client details')}
          visible={open}
          onCancel={() => setOpen(false)}
          footer={null}
          width='min(600px, calc(100vw - 32px))'
          bodyStyle={{ maxHeight: '70vh', overflow: 'auto' }}
        >
          <p style={{ color: 'var(--semi-color-text-2)', marginBottom: 16 }}>
            {t(
              'UA can be spoofed. Classification and admission do not verify official identity or guarantee cache hit rates.',
            )}
          </p>
          <dl
            style={{
              display: 'grid',
              gridTemplateColumns: 'auto minmax(0,1fr)',
              gap: 12,
            }}
          >
            <dt>{t('Client name')}</dt>
            <dd>{t(client.display_name)}</dd>
            <dt>{t('Client variant')}</dt>
            <dd>
              {t(`client.variant.${client.variant}`, {
                defaultValue: client.variant,
              })}
            </dd>
            <dt>{t('Version')}</dt>
            <dd>{client.version || t('Not recorded')}</dd>
            <dt>{t('Recognition source')}</dt>
            <dd>
              {t(
                client.source === 'generic_go_ua'
                  ? 'Generic Go UA inference'
                  : client.confidence === 'unknown'
                    ? 'Unrecognized UA'
                    : 'Explicit UA prefix',
              )}
            </dd>
          </dl>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              marginTop: 16,
            }}
          >
            <span>{t('Original User-Agent')}</span>
            <Tooltip content={t('Copy User-Agent')}>
              <Button
                aria-label={t('Copy User-Agent')}
                icon={<Copy size={16} />}
                theme='borderless'
                onClick={async () => {
                  if (await copy(client.user_agent)) Toast.success(t('Copied'));
                  else Toast.error(t('Copy failed'));
                }}
              />
            </Tooltip>
          </div>
          <pre
            dir='ltr'
            style={{
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
              maxHeight: 192,
              overflow: 'auto',
              padding: 12,
              background: 'var(--semi-color-fill-0)',
              borderRadius: 4,
            }}
          >
            {client.user_agent || t('Empty User-Agent')}
          </pre>
          {client.truncated && (
            <p>
              {t(
                'User-Agent truncated to 2048 bytes; control characters removed.',
              )}
            </p>
          )}
          {client.source === 'generic_go_ua' && (
            <p>
              {t(
                'NewAPI is a site convention for generic Go UA; the actual sender may be another Go client.',
              )}
            </p>
          )}
        </Modal>
      )}
    </div>
  );
}
