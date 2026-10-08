import { choosePasskeyDomain } from './passkeyDomainPicker';
import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Modal, Input, Select, Typography } from '@douyinfe/semi-ui';
import { t } from 'i18next';
import { useTranslation } from 'react-i18next';
import { API } from '../helpers/api';
import {
  buildAssertionResult,
  prepareCredentialRequestOptions,
} from '../helpers/passkey';

export async function verifyChallenge(info, method, value) {
  if (method === 'oauth') {
    const popup = window.open('about:blank', '_blank', 'width=560,height=700');
    if (!popup)
      throw new Error(t('Please allow popups to verify your account'));
    try {
      const begin = await API.get('/api/verify/oauth/begin', {
        params: { challenge: info.challenge, provider: value },
      });
      if (!begin.data.success) throw new Error(begin.data.message);
      popup.location.href = begin.data.data.url;
      const deadline = Date.now() + 180000;
      let verified = false;
      while (Date.now() < deadline) {
        await new Promise((resolve) => setTimeout(resolve, 2000));
        const result = await API.get('/api/verify/oauth/status', {
          headers: { 'X-Security-Challenge': info.challenge },
        });
        if (!result.data.success) throw new Error(result.data.message);
        if (result.data.data.verified) {
          verified = true;
          break;
        }
        if (popup.closed) throw new Error(t('Verification cancelled'));
      }
      if (!verified) throw new Error(t('Verification expired'));
    } finally {
      popup.close();
    }
  }
  if (method === 'passkey') {
    let begin = await API.post('/api/user/passkey/verify/begin', null, {
      headers: { 'X-Security-Challenge': info.challenge },
    });
    if (!begin.data.success) throw new Error(begin.data.message);
    if (begin.data.data.rp_ids?.length > 1) {
      const domain = await choosePasskeyDomain(begin.data.data.rp_ids);
      begin = await API.post('/api/user/passkey/verify/begin', null, {
        params: { rp_id: domain },
        headers: { 'X-Security-Challenge': info.challenge },
      });
      if (!begin.data.success) throw new Error(begin.data.message);
    }
    const credential = await navigator.credentials.get({
      publicKey: prepareCredentialRequestOptions(begin.data.data.options),
    });
    if (!credential) throw new Error(t('验证已取消'));
    const finish = await API.post(
      '/api/user/passkey/verify/finish',
      buildAssertionResult(credential),
    );
    if (!finish.data.success) throw new Error(finish.data.message);
  }
  const result = await API.post('/api/verify', {
    challenge: info.challenge,
    method,
    code: method === '2fa' ? value.trim() : undefined,
    password: method === 'password' ? value : undefined,
  });
  if (!result.data.success) throw new Error(result.data.message);
  return result.data.data.proof_token;
}

function ProofPrompt({ info, done }) {
  const { t } = useTranslation();
  const methods = info.methods;
  const [method, setMethod] = useState(methods[0] || '');
  const [value, setValue] = useState(
    info.methods[0] === 'oauth' ? (info.oauth_providers[0] ?? '') : '',
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const labels = {
    password: t('密码'),
    '2fa': t('验证码'),
    passkey: 'Passkey',
    oauth: 'OAuth',
  };
  const submit = async () => {
    if (busy || !method) return;
    setBusy(true);
    setError('');
    try {
      done(await verifyChallenge(info, method, value));
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  };
  return (
    <Modal
      visible
      title={t('安全验证')}
      style={{ maxWidth: 'calc(100vw - 32px)', width: 420 }}
      onCancel={() => {
        if (!busy) done();
      }}
      onOk={submit}
      confirmLoading={busy}
      okButtonProps={{ disabled: !method || (method !== 'passkey' && !value) }}
      cancelButtonProps={{ disabled: busy }}
    >
      <form
        style={{ display: 'grid', gap: 16 }}
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        {methods.length > 1 && (
          <Select
            aria-label={t('验证方式')}
            value={method}
            disabled={busy}
            optionList={methods.map((item) => ({
              value: item,
              label: labels[item],
            }))}
            onChange={(item) => {
              setMethod(item);
              setValue(item === 'oauth' ? (info.oauth_providers[0] ?? '') : '');
            }}
          />
        )}
        {method === 'oauth' && (
          <select
            aria-label={t('OAuth provider')}
            value={value}
            disabled={busy}
            onChange={(event) => setValue(event.target.value)}
          >
            {info.oauth_providers.map((provider) => (
              <option key={provider} value={provider}>
                {provider}
              </option>
            ))}
          </select>
        )}
        {method && method !== 'passkey' && method !== 'oauth' && (
          <Input
            aria-label={labels[method]}
            placeholder={labels[method]}
            autoFocus
            mode={method === 'password' ? 'password' : undefined}
            autoComplete={
              method === 'password' ? 'current-password' : 'one-time-code'
            }
            value={value}
            disabled={busy}
            onChange={setValue}
          />
        )}
        {!method && <Typography.Text>{t('无可用验证方式')}</Typography.Text>}
        {error && (
          <Typography.Text type='danger' role='alert'>
            {error}
          </Typography.Text>
        )}
      </form>
    </Modal>
  );
}

let queue = Promise.resolve();
export function requestSecurityProof(info) {
  const next = queue.then(
    () =>
      new Promise((resolve, reject) => {
        const host = document.createElement('div');
        document.body.appendChild(host);
        const root = createRoot(host);
        const done = (token) => {
          root.unmount();
          host.remove();
          if (token) resolve(token);
          else reject(new Error(t('验证已取消')));
        };
        root.render(<ProofPrompt info={info} done={done} />);
      }),
  );
  queue = next.catch(() => undefined);
  return next;
}
