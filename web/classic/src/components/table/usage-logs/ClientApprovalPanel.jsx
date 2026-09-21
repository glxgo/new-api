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
import React, { useEffect, useState } from 'react';
import {
  Button,
  Checkbox,
  Input,
  Modal,
  Pagination,
  Select,
  Spin,
  Tabs,
  TabPane,
  Toast,
  Tooltip,
} from '@douyinfe/semi-ui';
import { Check, History, RotateCcw, Save, ShieldCheck, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API } from '../../../helpers';

async function request(path, data, method = 'post') {
  const result =
    data === undefined
      ? await API.get(`/api/clients${path}`)
      : await API.request({ url: `/api/clients${path}`, method, data });
  if (!result.data.success) throw new Error(result.data.message);
  return result.data.data;
}

function Audit({ path }) {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [data, setData] = useState(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let active = true;
    setData(null);
    setFailed(false);
    request(`${path}&p=${page}&page_size=10`)
      .then((value) => {
        if (active) setData(value);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [path, page]);
  if (failed) return <p role='alert'>{t('Failed to load records')}</p>;
  if (!data) return <Spin />;
  return (
    <div>
      {data.items.map((item) => (
        <div
          key={item.id}
          style={{
            padding: '12px 0',
            borderBottom: '1px solid var(--semi-color-border)',
            overflowWrap: 'anywhere',
          }}
        >
          <p>
            {new Date(item.created_at * 1000).toLocaleString()} ·{' '}
            {t('Operator')} #{item.operator_id} ·{' '}
            {item.status && t(`client.status.${item.status}`)}
          </p>
          <p>{item.reason}</p>
          {item.policy && (
            <details>
              <summary>{t('Group policy')}</summary>
              <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                {item.previous_policy}
                {'\n'}
                {item.policy}
              </pre>
            </details>
          )}
        </div>
      ))}
      {data.total === 0 && <p>{t('No records')}</p>}
      <Pagination
        currentPage={page}
        pageSize={10}
        total={data.total}
        onPageChange={setPage}
      />
    </div>
  );
}

export default function ClientApprovalPanel() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [tab, setTab] = useState('pending');
  const [page, setPage] = useState(1);
  const [data, setData] = useState(null);
  const [failed, setFailed] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const [review, setReview] = useState(null);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [audit, setAudit] = useState('');
  const [groups, setGroups] = useState([]);
  const [policies, setPolicies] = useState([]);
  const [approved, setApproved] = useState([]);
  const [draft, setDraft] = useState(null);
  useEffect(() => {
    if (!open) return;
    let active = true;
    setData(null);
    setFailed(false);
    const load = async () => {
      if (tab !== 'groups')
        return request(`?status=${tab}&p=${page}&page_size=20`);
      const response = await API.get('/api/group/');
      if (!response.data.success) throw new Error('groups');
      const nextPolicies = await request('/groups');
      const clients = [];
      for (let p = 1; ; p++) {
        const result = await request(`?status=approved&p=${p}&page_size=100`);
        clients.push(...result.items);
        if (clients.length >= result.total || result.items.length === 0) break;
      }
      if (active) {
        setGroups(response.data.data);
        setPolicies(nextPolicies);
        setApproved(clients);
      }
      return { items: [], total: 0 };
    };
    load()
      .then((value) => {
        if (active) setData(value);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [open, tab, page, refresh]);
  const save = async () => {
    if (!reason.trim()) return;
    setBusy(true);
    try {
      if (review)
        await request('/review', {
          client_key: review.client.client_key,
          revision: review.client.revision,
          status: review.status,
          reason: reason.trim(),
        });
      else await request('/groups', { ...draft, reason: reason.trim() }, 'put');
      setReview(null);
      setDraft(null);
      setReason('');
      setRefresh((value) => value + 1);
      Toast.success(t('Saved'));
    } catch {
      Toast.error(t('Review failed. Refresh the list and try again.'));
    } finally {
      setBusy(false);
    }
  };
  const options = new Map(
    approved.map((item) => [item.client_key, item.display_name]),
  );
  draft?.supported_clients?.forEach((key) => {
    if (!options.has(key)) options.set(key, key);
  });
  return (
    <>
      <Button
        size='small'
        icon={<ShieldCheck size={14} />}
        onClick={() => setOpen(true)}
      >
        {t('Client approvals')}
      </Button>
      <Modal
        visible={open}
        onCancel={() => setOpen(false)}
        title={t('Coding client allowlist')}
        footer={null}
        width='min(900px, calc(100vw - 32px))'
        bodyStyle={{ maxHeight: '75vh', overflow: 'auto' }}
      >
        <p style={{ marginBottom: 16, color: 'var(--semi-color-text-2)' }}>
          {t(
            'UA can be spoofed. Classification and admission do not verify official identity or guarantee cache hit rates.',
          )}
        </p>
        <Tabs
          activeKey={tab}
          onChange={(value) => {
            setTab(value);
            setPage(1);
            setDraft(null);
            setReason('');
          }}
        >
          {['pending', 'approved', 'rejected', 'groups'].map((key) => (
            <TabPane key={key} itemKey={key} tab={t(`client.status.${key}`)} />
          ))}
        </Tabs>
        {failed ? (
          <p role='alert'>
            {t('Failed to load records')}
            <Button
              icon={<RotateCcw size={16} />}
              aria-label={t('Refresh')}
              onClick={() => setRefresh((value) => value + 1)}
            />
          </p>
        ) : !data ? (
          <Spin />
        ) : tab === 'groups' ? (
          <div style={{ display: 'grid', gap: 16, marginTop: 16 }}>
            <Select
              aria-label={t('Group')}
              placeholder={t('Select group')}
              value={draft?.group_name}
              optionList={groups.map((value) => ({ value, label: value }))}
              onChange={(group) => {
                const policy = policies.find(
                  (item) => item.group_name === group,
                );
                setDraft(
                  policy
                    ? {
                        ...policy,
                        supported_clients: policy.supported_clients || [],
                      }
                    : {
                        group_name: group,
                        is_coding: group.toLowerCase().includes('coding'),
                        supported_clients: [],
                        revision: 0,
                      },
                );
                setReason('');
              }}
            />
            {draft && (
              <>
                <Checkbox
                  checked={draft.is_coding}
                  onChange={(e) =>
                    setDraft({ ...draft, is_coding: e.target.checked })
                  }
                >
                  {t('Coding group')}
                </Checkbox>
                <fieldset disabled={!draft.is_coding}>
                  <legend>{t('Supported clients')}</legend>
                  <div
                    style={{
                      display: 'grid',
                      gap: 8,
                      maxHeight: 240,
                      overflow: 'auto',
                      padding: 4,
                    }}
                  >
                    {[...options].map(([key, name]) => (
                      <Checkbox
                        key={key}
                        checked={draft.supported_clients.includes(key)}
                        onChange={(e) =>
                          setDraft({
                            ...draft,
                            supported_clients: e.target.checked
                              ? [...draft.supported_clients, key]
                              : draft.supported_clients.filter(
                                  (value) => value !== key,
                                ),
                          })
                        }
                      >
                        {t(name)} <small>{key}</small>
                      </Checkbox>
                    ))}
                  </div>
                </fieldset>
                <label htmlFor='classic-client-policy-reason'>
                  {t('Reason (required)')}
                </label>
                <Input.TextArea
                  id='classic-client-policy-reason'
                  value={reason}
                  maxCount={500}
                  onChange={setReason}
                />
                <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                  <Button
                    icon={<History size={16} />}
                    onClick={() =>
                      setAudit(
                        `/group-reviews?group_name=${encodeURIComponent(draft.group_name)}`,
                      )
                    }
                  >
                    {t('Approval history')}
                  </Button>
                  <Button
                    icon={<Save size={16} />}
                    disabled={!reason.trim() || busy}
                    onClick={save}
                  >
                    {t('Save group policy')}
                  </Button>
                </div>
              </>
            )}
          </div>
        ) : (
          <>
            {data.items.map((client) => (
              <div
                key={client.client_key}
                style={{
                  padding: '12px 0',
                  borderBottom: '1px solid var(--semi-color-border)',
                  minWidth: 0,
                }}
              >
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    gap: 8,
                    flexWrap: 'wrap',
                  }}
                >
                  <div>
                    <b>{t(client.display_name)}</b>
                    <p style={{ fontSize: 12, overflowWrap: 'anywhere' }}>
                      {client.client_key}
                    </p>
                  </div>
                  <div style={{ display: 'flex', gap: 4 }}>
                    {[
                      ['approved', 'Approve client', Check],
                      ['rejected', 'Reject client', X],
                      ['pending', 'Revoke approval', RotateCcw],
                    ]
                      .filter(([status]) => status !== client.status)
                      .map(([status, label, Icon]) => (
                        <Tooltip key={status} content={t(label)}>
                          <Button
                            theme='borderless'
                            aria-label={t(label)}
                            disabled={
                              status === 'approved' &&
                              client.family === 'unknown'
                            }
                            icon={<Icon size={16} />}
                            onClick={() => {
                              setReview({ client, status });
                              setReason('');
                            }}
                          />
                        </Tooltip>
                      ))}
                    <Tooltip content={t('Approval history')}>
                      <Button
                        theme='borderless'
                        aria-label={t('Approval history')}
                        icon={<History size={16} />}
                        onClick={() =>
                          setAudit(
                            `/reviews?client_key=${encodeURIComponent(client.client_key)}`,
                          )
                        }
                      />
                    </Tooltip>
                  </div>
                </div>
                <p style={{ fontSize: 12 }}>
                  {t('Requests')}: {client.request_count} · {t('Last seen')}:{' '}
                  {new Date(client.last_seen * 1000).toLocaleString()}
                </p>
                <pre
                  dir='ltr'
                  style={{
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-all',
                    maxHeight: 96,
                    overflow: 'auto',
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
              </div>
            ))}
            {data.total === 0 && <p>{t('No records')}</p>}
            <Pagination
              total={data.total}
              pageSize={20}
              currentPage={page}
              onPageChange={setPage}
            />
          </>
        )}
      </Modal>
      <Modal
        visible={!!review}
        title={t('Review client')}
        onCancel={() => {
          if (!busy) setReview(null);
        }}
        onOk={save}
        confirmLoading={busy}
        okButtonProps={{ disabled: !reason.trim() }}
        okText={t('Save review')}
        width='min(520px, calc(100vw - 32px))'
      >
        <p>
          {review && t(review.client.display_name)} ·{' '}
          {review && t(`client.status.${review.status}`)}
        </p>
        <label htmlFor='classic-client-review-reason'>
          {t('Reason (required)')}
        </label>
        <Input.TextArea
          id='classic-client-review-reason'
          value={reason}
          maxCount={500}
          onChange={setReason}
        />
      </Modal>
      <Modal
        visible={!!audit}
        title={t('Approval history')}
        onCancel={() => setAudit('')}
        footer={null}
        width='min(600px, calc(100vw - 32px))'
        bodyStyle={{ maxHeight: '70vh', overflow: 'auto' }}
      >
        {audit && <Audit key={audit} path={audit} />}
      </Modal>
    </>
  );
}
