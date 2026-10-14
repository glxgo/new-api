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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Col,
  Form,
  Input,
  Row,
  Select,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess, showWarning } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;
const fixedPoint = /^[0-9]{1,18}(\.[0-9]{1,9})?$/;

const emptyDraft = {
  mode: 'off',
  groups: '',
  pricePerCredit: '0.11',
  costPerCredit: '0',
  quotaPerCurrency: '500000',
  reserveCredits: '10',
  trustedChannels: '',
  channelCosts: [],
};

function listFromText(value) {
  return [
    ...new Set(
      value
        .split(/[\n,]/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  ];
}

function parseDraft(value) {
  try {
    const raw = JSON.parse(value || '{}');
    return {
      mode: ['shadow', 'charge'].includes(raw.mode) ? raw.mode : 'off',
      groups: Array.isArray(raw.groups) ? raw.groups.join('\n') : '',
      pricePerCredit: String(raw.price_per_credit ?? emptyDraft.pricePerCredit),
      costPerCredit: String(raw.cost_per_credit ?? emptyDraft.costPerCredit),
      quotaPerCurrency: String(
        raw.quota_per_currency ?? emptyDraft.quotaPerCurrency,
      ),
      reserveCredits: String(raw.reserve_credits ?? emptyDraft.reserveCredits),
      trustedChannels: Array.isArray(raw.trusted_channels)
        ? raw.trusted_channels.join(', ')
        : '',
      channelCosts:
        raw.channel_cost_per_credit &&
        typeof raw.channel_cost_per_credit === 'object'
          ? Object.entries(raw.channel_cost_per_credit).map(
              ([channelId, cost]) => ({ channelId, cost: String(cost) }),
            )
          : [],
    };
  } catch {
    return { ...emptyDraft };
  }
}

function buildConfig(draft, savedValue) {
  if (draft.mode === 'off')
    return { ...JSON.parse(savedValue || '{}'), mode: 'off' };
  return {
    mode: draft.mode,
    groups: listFromText(draft.groups),
    price_per_credit: draft.pricePerCredit.trim(),
    cost_per_credit: draft.costPerCredit.trim(),
    quota_per_currency: Number(draft.quotaPerCurrency),
    reserve_credits: draft.reserveCredits.trim(),
    trusted_channels: listFromText(draft.trustedChannels).map(Number),
    channel_cost_per_credit: Object.fromEntries(
      draft.channelCosts
        .filter((row) => row.channelId.trim() || row.cost.trim())
        .map((row) => [String(Number(row.channelId)), row.cost.trim()]),
    ),
  };
}

function validateDraft(draft, t) {
  if (draft.mode === 'off') return true;
  if (!listFromText(draft.groups).length) {
    showWarning(t('请至少填写一个服务分组'));
    return false;
  }
  if (listFromText(draft.groups).includes('auto')) {
    showWarning(t('分组名必须与渠道服务分组完全一致，不能填写 auto。'));
    return false;
  }
  if (
    !listFromText(draft.trustedChannels).length ||
    listFromText(draft.trustedChannels).some(
      (id) =>
        !/^\d+$/.test(id) ||
        !Number.isSafeInteger(Number(id)) ||
        Number(id) <= 0,
    )
  ) {
    showWarning(t('可信渠道必须填写正整数 ID'));
    return false;
  }
  if (
    !fixedPoint.test(draft.pricePerCredit.trim()) ||
    !Number.isFinite(Number(draft.pricePerCredit)) ||
    Number(draft.pricePerCredit) <= 0
  ) {
    showWarning(t('用户售价必须大于 0'));
    return false;
  }
  if (
    !fixedPoint.test(draft.costPerCredit.trim()) ||
    !Number.isFinite(Number(draft.costPerCredit)) ||
    Number(draft.costPerCredit) < 0
  ) {
    showWarning(t('默认成本不能小于 0'));
    return false;
  }
  if (
    !Number.isSafeInteger(Number(draft.quotaPerCurrency)) ||
    Number(draft.quotaPerCurrency) <= 0
  ) {
    showWarning(t('额度换算必须是正整数'));
    return false;
  }
  if (
    !fixedPoint.test(draft.reserveCredits.trim()) ||
    !Number.isFinite(Number(draft.reserveCredits)) ||
    Number(draft.reserveCredits) <= 0
  ) {
    showWarning(t('预扣 credit 必须大于 0'));
    return false;
  }
  const ids = new Set();
  for (const row of draft.channelCosts) {
    if (!row.channelId.trim() && !row.cost.trim()) continue;
    if (!row.channelId.trim() || !row.cost.trim()) {
      showWarning(t('渠道 ID 和成本必须同时填写'));
      return false;
    }
    if (
      !/^\d+$/.test(row.channelId.trim()) ||
      !Number.isSafeInteger(Number(row.channelId)) ||
      Number(row.channelId) <= 0 ||
      ids.has(Number(row.channelId))
    ) {
      showWarning(t('渠道成本覆盖中的 ID 必须为不重复的正整数'));
      return false;
    }
    ids.add(Number(row.channelId));
    if (
      !fixedPoint.test(row.cost.trim()) ||
      !Number.isFinite(Number(row.cost)) ||
      Number(row.cost) < 0
    ) {
      showWarning(t('渠道成本不能小于 0'));
      return false;
    }
  }
  return true;
}

export default function SettingsKiroCreditBilling({ options, refresh }) {
  const { t } = useTranslation();
  const initialDraft = useMemo(
    () => parseDraft(options?.KiroCreditBilling),
    [options?.KiroCreditBilling],
  );
  const [draft, setDraft] = useState(initialDraft);
  const [saved, setSaved] = useState(JSON.stringify(initialDraft));
  const [loading, setLoading] = useState(false);
  const [formApi, setFormApi] = useState(null);

  useEffect(() => {
    setDraft(initialDraft);
    setSaved(JSON.stringify(initialDraft));
  }, [initialDraft]);

  const update = (field, value) =>
    setDraft((current) => ({ ...current, [field]: value }));
  const dirty = JSON.stringify(draft) !== saved;

  const missingGroupRatio = useMemo(() => {
    const groups = listFromText(draft.groups);
    if (!groups.length || !options?.GroupRatio) return false;
    try {
      const ratios = JSON.parse(options.GroupRatio);
      return groups.some(
        (group) => !Object.prototype.hasOwnProperty.call(ratios, group),
      );
    } catch {
      return false;
    }
  }, [draft.groups, options?.GroupRatio]);

  const onSave = async () => {
    if (!validateDraft(draft, t)) return;
    setLoading(true);
    try {
      const res = await API.put('/api/option/', {
        key: 'KiroCreditBilling',
        value: JSON.stringify(buildConfig(draft, options?.KiroCreditBilling)),
      });
      if (res.data.success) {
        showSuccess(t('保存成功'));
        setSaved(JSON.stringify(draft));
        await refresh();
      } else {
        showError(res.data.message || t('保存失败，请重试'));
      }
    } catch {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  const addOverride = () =>
    update('channelCosts', [
      ...draft.channelCosts,
      { channelId: '', cost: '' },
    ]);
  const removeOverride = (index) =>
    update(
      'channelCosts',
      draft.channelCosts.filter((_, current) => current !== index),
    );

  return (
    <Spin spinning={loading}>
      <Form
        key={JSON.stringify(initialDraft)}
        initValues={initialDraft}
        getFormApi={setFormApi}
      >
        <Form.Section text={t('Kiro Credit 计费')}>
          {draft.mode === 'charge' && (
            <Banner
              type='danger'
              description={t(
                'Charge 模式会从用户余额结算每次匹配的 Kiro 请求，请先用 Shadow 模式核对回执和账本。',
              )}
              closeIcon={null}
              className='!rounded-lg mb-3'
            />
          )}
          {draft.mode === 'shadow' && (
            <Banner
              type='info'
              description={t(
                'Shadow 记录 Kiro credit 用于对照，原有请求计费仍然生效，不会额外按 credit 扣款。',
              )}
              closeIcon={null}
              className='!rounded-lg mb-3'
            />
          )}
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8}>
              <Form.Select
                field='mode'
                label={t('计费模式')}
                value={draft.mode}
                onChange={(value) => update('mode', value)}
              >
                <Select.Option value='off'>{t('关闭计费')}</Select.Option>
                <Select.Option value='shadow'>
                  {t('Shadow 只记录')}
                </Select.Option>
                <Select.Option value='charge'>
                  {t('Charge 实际结算')}
                </Select.Option>
              </Form.Select>
            </Col>
            <Col xs={24} sm={12} md={8}>
              <Form.Input
                field='pricePerCredit'
                label={t('用户售价（CNY / credit）')}
                value={draft.pricePerCredit}
                onChange={(value) => update('pricePerCredit', value)}
                disabled={draft.mode === 'off'}
              />
            </Col>
            <Col xs={24} sm={12} md={8}>
              <Form.Input
                field='costPerCredit'
                label={t('默认渠道成本（CNY / credit）')}
                value={draft.costPerCredit}
                onChange={(value) => update('costPerCredit', value)}
                disabled={draft.mode === 'off'}
              />
            </Col>
            <Col xs={24} sm={12} md={8}>
              <Form.InputNumber
                field='quotaPerCurrency'
                label={t('每 CNY 额度换算')}
                value={Number(draft.quotaPerCurrency)}
                onChange={(value) =>
                  update('quotaPerCurrency', String(value ?? ''))
                }
                min={1}
                step={1}
                disabled={draft.mode === 'off'}
              />
            </Col>
            <Col xs={24} sm={12} md={8}>
              <Form.Input
                field='reserveCredits'
                label={t('每次请求预扣 credit')}
                value={draft.reserveCredits}
                onChange={(value) => update('reserveCredits', value)}
                disabled={draft.mode === 'off'}
              />
            </Col>
            <Col xs={24}>
              <Form.TextArea
                field='groups'
                label={t('服务分组')}
                value={draft.groups}
                onChange={(value) => update('groups', value)}
                placeholder={t('每行填写一个精确的分组名')}
                autosize={{ minRows: 3, maxRows: 6 }}
                disabled={draft.mode === 'off'}
              />
              <Text type='tertiary'>
                {t('分组名必须与渠道服务分组完全一致，不能填写 auto。')}
              </Text>
            </Col>
            {missingGroupRatio && (
              <Col xs={24}>
                <Banner
                  type='warning'
                  description={t(
                    '至少一个分组没有精确的 GroupRatio 配置，后端会按 1 倍率计算。',
                  )}
                  closeIcon={null}
                />
              </Col>
            )}
            <Col xs={24}>
              <Form.TextArea
                field='trustedChannels'
                label={t('可信渠道 ID')}
                value={draft.trustedChannels}
                onChange={(value) => update('trustedChannels', value)}
                placeholder={t('例如：19, 89')}
                autosize={{ minRows: 2, maxRows: 4 }}
                disabled={draft.mode === 'off'}
              />
              <Text type='tertiary'>
                {t('只有这些渠道 ID 会参与 Shadow 或 Charge。')}
              </Text>
            </Col>
            <Col xs={24}>
              <div style={{ marginBottom: 8 }}>
                <Text strong>{t('渠道成本覆盖')}</Text>
                <br />
                <Text type='tertiary'>
                  {t('可选；未填写的渠道使用默认成本。')}
                </Text>
              </div>
              {draft.channelCosts.map((row, index) => (
                <Row gutter={16} key={index} style={{ marginBottom: 8 }}>
                  <Col xs={10} sm={8}>
                    <Input
                      placeholder={t('渠道 ID')}
                      value={row.channelId}
                      onChange={(value) =>
                        update(
                          'channelCosts',
                          draft.channelCosts.map((item, current) =>
                            current === index
                              ? { ...item, channelId: value }
                              : item,
                          ),
                        )
                      }
                      disabled={draft.mode === 'off'}
                    />
                  </Col>
                  <Col xs={10} sm={8}>
                    <Input
                      placeholder={t('成本 / credit')}
                      value={row.cost}
                      onChange={(value) =>
                        update(
                          'channelCosts',
                          draft.channelCosts.map((item, current) =>
                            current === index ? { ...item, cost: value } : item,
                          ),
                        )
                      }
                      disabled={draft.mode === 'off'}
                    />
                  </Col>
                  <Col xs={4} sm={4}>
                    <Button
                      type='tertiary'
                      theme='borderless'
                      onClick={() => removeOverride(index)}
                      disabled={draft.mode === 'off'}
                    >
                      {t('删除')}
                    </Button>
                  </Col>
                </Row>
              ))}
              <Button
                type='tertiary'
                theme='borderless'
                onClick={addOverride}
                disabled={draft.mode === 'off'}
              >
                {t('添加渠道成本覆盖')}
              </Button>
            </Col>
          </Row>
          <Row style={{ marginTop: 16 }}>
            <Button theme='solid' onClick={onSave} disabled={!dirty || loading}>
              {t('保存 Kiro 设置')}
            </Button>
            <Button
              style={{ marginLeft: 8 }}
              onClick={() => {
                setDraft(initialDraft);
                formApi?.setValues(initialDraft);
              }}
              disabled={!dirty || loading}
            >
              {t('重置')}
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}
