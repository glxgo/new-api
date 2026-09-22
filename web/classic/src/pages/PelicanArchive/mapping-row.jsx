import React, { useRef, useState } from 'react';
import { Form, Button, Tag } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

const reasons = {
  source_removed: '来源目标已移除',
  source_disabled: '来源目标已停用',
  target_hidden: '测试展示已隐藏',
  unmapped: '未关联',
  channel_missing: '关联渠道已不存在',
  channel_disabled: '关联渠道已停用',
  model_unavailable: '关联模型已不受支持',
  group_unavailable: '分组尚未登记',
  group_hidden: '分组展示已隐藏',
  ability_unavailable: '该分组的渠道模型不可用',
  no_eligible_group: '暂无可展示分组',
};

export default function MappingRow({
  target,
  channels,
  report,
  disabled,
  save,
}) {
  const { t } = useTranslation();
  const initial = {
    channel_id: target.channel_id,
    display_model: target.display_model,
    hidden: target.hidden,
  };
  const [draft, setDraft] = useState(initial);
  const form = useRef(null);
  const models = (channels.find((c) => c.id === draft.channel_id)?.models || '')
    .split(',')
    .map((m) => m.trim())
    .filter(Boolean);
  const missing =
    draft.channel_id && !channels.some((c) => c.id === draft.channel_id);
  const dirty =
    draft.channel_id !== target.channel_id ||
    draft.display_model !== target.display_model ||
    draft.hidden !== target.hidden;
  return (
    <section className='pelican-mapping-row'>
      <h4>
        {target.provider_name} · {target.model_name}
      </h4>
      <p className='pelican-evidence-note'>{target.provider_id}</p>
      <Form
        initValues={initial}
        getFormApi={(api) => {
          form.current = api;
        }}
        onValueChange={setDraft}
        onSubmit={(values) => save({ target_id: target.id, ...values })}
      >
        <Form.Select
          field='channel_id'
          label={t('本站渠道')}
          disabled={disabled}
          onChange={() => form.current?.setValue('display_model', '')}
          optionList={[
            { label: t('未关联'), value: 0 },
            ...(missing
              ? [
                  {
                    label: `#${draft.channel_id} · ${t('关联渠道已不存在')}`,
                    value: draft.channel_id,
                  },
                ]
              : []),
            ...channels.map((c) => ({
              label: `#${c.id} ${c.name}`,
              value: c.id,
            })),
          ]}
        />
        <Form.Select
          field='display_model'
          label={t('本站模型（须存在于所选渠道）')}
          disabled={disabled || !draft.channel_id}
          optionList={models.map((m) => ({ label: m, value: m }))}
        />
        <Form.Switch
          field='hidden'
          label={t('隐藏此目标')}
          disabled={disabled}
        />
        <Button
          htmlType='submit'
          disabled={
            disabled ||
            (!!draft.channel_id && !models.includes(draft.display_model))
          }
        >
          {t('保存关联')}
        </Button>
      </Form>
      {report && (
        <div className='pelican-mapping-report'>
          <div className='pelican-evidence-summary'>
            <strong>{t('已保存关联')}</strong>
            <Tag>
              {report.reason
                ? t(reasons[report.reason] || '关联需要核对')
                : t('关联有效')}
            </Tag>
          </div>
          {dirty && (
            <p className='pelican-evidence-note'>
              {t('未保存的修改不影响下方核对结果。')}
            </p>
          )}
          {report.channel_disabled && (
            <p className='pelican-evidence-note'>
              {t('业务渠道已停用，外部测试记录仍可按当前分组归属参与展示。')}
            </p>
          )}
          {report.groups.map((g) => (
            <div key={g.routing_key} className='pelican-actions'>
              <span>
                {g.display_name} · {g.ratio ?? '—'}×
              </span>
              <span className='pelican-evidence-note'>
                {g.eligible
                  ? t('可参与精选展示')
                  : t(reasons[g.reason] || '关联需要核对')}
              </span>
            </div>
          ))}
          <p className='pelican-evidence-note'>
            {t(
              '分组随渠道当前归属更新；实际展示还受页面开关、用户权限与精选规则影响。',
            )}
          </p>
        </div>
      )}
    </section>
  );
}
