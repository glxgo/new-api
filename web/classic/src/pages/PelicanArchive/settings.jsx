import React, { useEffect, useState } from 'react';
import {
  Card,
  Button,
  Tabs,
  TabPane,
  Form,
  Table,
  Banner,
  Collapse,
  Input,
  Switch,
  Space,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, isRoot } from '../../helpers';
import PelicanArchive, { archiveBase, Evidence, time } from './index';
import MappingRow from './mapping-row';

const copySlots = [
  ['page_title', '概览标题'],
  ['page_intro', '概览说明'],
  ['gallery_title', '作品说明'],
  ['empty_text', '无结果说明'],
  ['method_intro', '方法简介'],
  ['method_scope_title', '第一章标题'],
  ['method_evidence_title', '第二章标题'],
  ['method_selection_title', '第三章标题'],
  ['method_reading_title', '第四章标题'],
  ['method_trace_title', '第五章标题'],
  ['method_limits_title', '第六章标题'],
];
export default function PelicanSettings() {
  const { t } = useTranslation();
  const [data, setData] = useState(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [page, setPage] = useState(1),
    [records, setRecords] = useState({ data: [], total: 0 }),
    [detail, setDetail] = useState(null);
  const load = async () => {
    try {
      const r = await API.get(archiveBase + '/admin/control');
      if (!r.data.success) throw new Error(r.data.message);
      setData(r.data.data);
      setError('');
    } catch (e) {
      setError(e.message);
    }
  };
  useEffect(() => {
    load();
    const timer = setInterval(load, 30000);
    return () => clearInterval(timer);
  }, []);
  useEffect(() => {
    let active = true;
    API.get(`${archiveBase}/admin/records?p=${page}&page_size=20`)
      .then((r) => {
        if (active && r.data.success) setRecords(r.data);
      })
      .catch(() => showError(t('无法读取记录')));
    return () => {
      active = false;
    };
  }, [page, data]);
  const act = async (action, payload = {}) => {
    setBusy(true);
    try {
      const r = await API.put(archiveBase + '/admin/control', {
        revision: data.control.revision,
        action,
        ...payload,
      });
      if (!r.data.success) throw new Error(r.data.message);
      await load();
      showSuccess(t('已保存'));
    } catch (e) {
      showError(e.message);
    } finally {
      setBusy(false);
    }
  };
  if (!data) return <Banner description={error || t('正在读取存档设置')} />;
  const disabled = busy || !isRoot() || !data.writable_node;
  const sync = async () => {
    setBusy(true);
    try {
      const r = await API.post(archiveBase + '/admin/sync');
      if (!r.data.success) throw new Error(r.data.message);
      await load();
      showSuccess(t('同步完成'));
    } catch (e) {
      showError(e.message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <Tabs>
      <TabPane tab={t('存档设置')} itemKey='settings'>
        <div className='pelican-stack'>
          <Card title={t('鹈鹕存档来源')}>
            <p>
              {t('本站只同步已有结果。下列操作不会启动或停止外部网站的测试。')}
            </p>
            <p>
              {t('最近同步')}：{time(data.control.last_imported_at)} ·{' '}
              {t('来源周期')}：{data.source_config?.interval_minutes ?? '—'}{' '}
              {t('分钟')} · {t('标准答案')}：
              {data.source_config?.expected_answer ?? '—'}
            </p>
            {data.control.last_error && (
              <Banner type='danger' description={data.control.last_error} />
            )}
            <p>
              {t('存档获取时间')}：
              {data.control.last_captured_at
                ? new Date(data.control.last_captured_at).toLocaleString()
                : '—'}
            </p>
            <Space wrap>
              <Button
                disabled={disabled}
                onClick={() =>
                  act(data.control.sync_enabled ? 'pause' : 'resume')
                }
              >
                {data.control.sync_enabled ? t('暂停同步') : t('恢复同步')}
              </Button>
              <Button
                disabled={
                  disabled ||
                  !data.source_configured ||
                  !data.control.sync_enabled
                }
                onClick={sync}
              >
                {t('立即同步')}
              </Button>
              <Button disabled={disabled} onClick={() => act('hide_and_pause')}>
                {t('隐藏并暂停同步')}
              </Button>
              <Button
                disabled={disabled}
                onClick={() => act(data.control.visible ? 'hide' : 'show')}
              >
                {data.control.visible ? t('仅隐藏') : t('显示页面')}
              </Button>
            </Space>
            <Form
              key={data.control.interval_minutes}
              initValues={{ interval: data.control.interval_minutes }}
              onSubmit={(v) =>
                act('interval', { interval_minutes: v.interval })
              }
            >
              <Form.InputNumber
                field='interval'
                label={t('同步间隔（分钟）')}
                min={1}
                max={1440}
                disabled={disabled}
              />
              <Button htmlType='submit' disabled={disabled}>
                {t('保存间隔')}
              </Button>
            </Form>
          </Card>
          <Card title={t('渠道关联')}>
            <p>
              {t('按稳定身份关联，修改名称不会改动归属；隐藏只影响本站展示。')}
            </p>
            {data.targets.map((target) => (
              <MappingRow
                key={`${target.id}:${target.channel_id}:${target.display_model}:${target.hidden}`}
                target={target}
                channels={data.channels}
                report={data.mapping_reports?.find(
                  (r) => r.target_id === target.id,
                )}
                disabled={disabled}
                save={(payload) => act('mapping', payload)}
              />
            ))}
          </Card>
          <Presentation
            key={JSON.stringify(data.presentation)}
            data={data}
            disabled={disabled}
            save={(presentation) => act('presentation', { presentation })}
          />
          <Collapse>
            <Collapse.Panel itemKey='events' header={t('管理与同步事件')}>
              <Table
                pagination={false}
                dataSource={data.events}
                rowKey='id'
                columns={[
                  { title: t('时间'), dataIndex: 'at', render: time },
                  { title: t('操作'), dataIndex: 'action' },
                  { title: t('操作者'), dataIndex: 'actor_id' },
                ]}
              />
            </Collapse.Panel>
          </Collapse>
        </div>
      </TabPane>
      <TabPane tab={t('来源记录')} itemKey='records'>
        <Table
          pagination={false}
          dataSource={records.data}
          rowKey='id'
          columns={[
            { title: t('记录'), dataIndex: 'external_id' },
            { title: t('时间'), dataIndex: 'tested_at', render: time },
            { title: t('结果'), dataIndex: 'grade' },
            {
              title: t('详情'),
              render: (_, r) => (
                <Button onClick={() => setDetail(r.id)}>{t('查看')}</Button>
              ),
            },
          ]}
        />
        <Space>
          <Button disabled={page === 1} onClick={() => setPage(page - 1)}>
            {t('上一页')}
          </Button>
          <span>
            {page} · {records.total}
          </span>
          <Button
            disabled={page * 20 >= records.total}
            onClick={() => setPage(page + 1)}
          >
            {t('下一页')}
          </Button>
        </Space>
        <Evidence id={detail} admin onClose={() => setDetail(null)} />
      </TabPane>
      <TabPane tab={t('用户预览')} itemKey='preview'>
        <Banner
          description={t(
            '管理员预览使用真实存档，不会公开页面或发起模型调用。',
          )}
        />
        <PelicanArchive preview />
      </TabPane>
    </Tabs>
  );
}
function Presentation({ data, disabled, save }) {
  const { t } = useTranslation();
  const [view, setView] = useState(data.presentation);
  const groups = [...data.groups].sort((a, b) => {
    const ai = view.order.indexOf(a.group_uid),
      bi = view.order.indexOf(b.group_uid);
    return (ai < 0 ? 100000 : ai) - (bi < 0 ? 100000 : bi);
  });
  const move = (i, d) => {
    const order = groups.map((g) => g.group_uid);
    [order[i], order[i + d]] = [order[i + d], order[i]];
    setView({ ...view, order });
  };
  return (
    <Card title={t('展示与文案')}>
      <div className='pelican-stack'>
        <Space>
          <Switch
            checked={view.show_method}
            disabled={disabled}
            onChange={(v) => setView({ ...view, show_method: v })}
          />
          {t('显示方法说明')}
          <Switch
            checked={view.show_history}
            disabled={disabled}
            onChange={(v) => setView({ ...view, show_history: v })}
          />
          {t('显示历史')}
        </Space>
        <Collapse>
          {groups.map((g, i) => {
            const v = view.groups[g.group_uid] || {
              name: null,
              description_mode: 'inherit',
              description: '',
              hidden: false,
            };
            const update = (delta) =>
              setView({
                ...view,
                groups: { ...view.groups, [g.group_uid]: { ...v, ...delta } },
              });
            return (
              <Collapse.Panel
                key={g.group_uid}
                itemKey={g.group_uid}
                header={`${v.name || g.display_name} · ${g.ratio ?? '—'}×`}
              >
                <div className='pelican-stack'>
                  <label>
                    {t('展示名称')}
                    <Input
                      value={v.name || ''}
                      placeholder={g.routing_key}
                      disabled={disabled}
                      maxLength={40}
                      onChange={(name) => update({ name: name || null })}
                    />
                  </label>
                  <label>
                    {t('展示描述')}
                    <Input
                      value={v.description}
                      placeholder={g.description}
                      disabled={disabled}
                      maxLength={160}
                      onChange={(description) =>
                        update({
                          description,
                          description_mode: description ? 'custom' : 'inherit',
                        })
                      }
                    />
                  </label>
                  <Space>
                    <Switch
                      checked={v.hidden}
                      disabled={disabled}
                      onChange={(hidden) => update({ hidden })}
                    />
                    {t('隐藏分组')}
                    <Button
                      disabled={disabled || i === 0}
                      onClick={() => move(i, -1)}
                    >
                      {t('上移')}
                    </Button>
                    <Button
                      disabled={disabled || i === groups.length - 1}
                      onClick={() => move(i, 1)}
                    >
                      {t('下移')}
                    </Button>
                  </Space>
                </div>
              </Collapse.Panel>
            );
          })}
        </Collapse>
        <Collapse>
          <Collapse.Panel
            itemKey='copy'
            header={t('文案管理（用户页展示位置）')}
          >
            <div className='pelican-stack'>
              {copySlots.map(([key, label]) => (
                <label key={key}>
                  {t(label)}
                  <Input
                    value={view.copy[key] || ''}
                    maxLength={500}
                    disabled={disabled}
                    onChange={(text) =>
                      setView({ ...view, copy: { ...view.copy, [key]: text } })
                    }
                  />
                </label>
              ))}
            </div>
          </Collapse.Panel>
        </Collapse>
        <Space>
          <Button disabled={disabled} onClick={() => save(view)}>
            {t('应用展示设置')}
          </Button>
          <Button
            disabled={disabled}
            onClick={() => setView(data.presentation)}
          >
            {t('放弃更改')}
          </Button>
          {JSON.stringify(view) !== JSON.stringify(data.presentation) && (
            <span>{t('有未保存更改')}</span>
          )}
        </Space>
      </div>
    </Card>
  );
}
