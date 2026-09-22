import React, { useEffect, useState } from 'react';
import {
  Card,
  Button,
  Tabs,
  TabPane,
  Form,
  Table,
  Tag,
  Typography,
  SideSheet,
  Banner,
  Space,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess, isRoot } from '../../helpers';
import CopyEditor from './copy-editor';

export default function IntelligenceSettings() {
  const [row, setRow] = useState(null);
  const [targets, setTargets] = useState([]);
  const [groups, setGroups] = useState([]);
  const [runs, setRuns] = useState([]);
  const [busy, setBusy] = useState(false);
  const [detail, setDetail] = useState(null);
  const load = async () => {
    try {
      const values = await Promise.all(
        ['/control', '/targets', '/groups', '/runs'].map((path) =>
          API.get(`/api/capability/admin${path}`),
        ),
      );
      if (values.some((v) => !v.data.success)) throw new Error('读取失败');
      setRow(values[0].data.data);
      setTargets(values[1].data.data);
      setGroups(values[2].data.data);
      setRuns(values[3].data.data);
    } catch {
      showError('无法读取智商测试设置');
    }
  };
  useEffect(() => {
    load();
  }, []);
  const save = async (action, payload = {}, revision = row.revision) => {
    setBusy(true);
    try {
      const { data } = await API.put('/api/capability/admin/control', {
        revision,
        action,
        ...payload,
      });
      if (!data.success) throw new Error(data.message);
      setRow(data.data);
      showSuccess('设置已保存');
      return data.data;
    } catch (e) {
      showError(e.message);
      throw e;
    } finally {
      setBusy(false);
    }
  };
  if (!row) return <Banner description='正在读取智商测试设置' />;
  const disabled = busy || !isRoot();
  const act = (action) => {
    save(action).catch(() => {});
  };
  const exclude = (target) => {
    const list = row.config.excluded;
    const exists = list.some(
      (e) => e.channel_id === target.channel_id && !e.group_uid && !e.model,
    );
    const excluded = exists
      ? list.filter(
          (e) =>
            !(e.channel_id === target.channel_id && !e.group_uid && !e.model),
        )
      : [...list, { channel_id: target.channel_id, group_uid: '', model: '' }];
    save('save_config', { config: { ...row.config, excluded } }).catch(
      () => {},
    );
  };
  return (
    <Card title='智商测试' id='capability-testing'>
      <Space wrap>
        <Tag>{row.visible ? '前台显示' : '前台隐藏'}</Tag>
        <Tag>{row.running ? '运行中' : '已停止'}</Tag>
        <Button
          type='danger'
          disabled={disabled}
          onClick={() => act('hide_and_stop')}
        >
          隐藏并停止测试
        </Button>
        <Button disabled={disabled} onClick={() => act('stop')}>
          停止全部
        </Button>
        <Button disabled={disabled} onClick={() => act('hide_only')}>
          仅隐藏，保持运行状态
        </Button>
        <Button disabled={disabled} onClick={() => act('show_only')}>
          显示测试
        </Button>
        <Button disabled={disabled} onClick={() => act('resume')}>
          恢复测试
        </Button>
      </Space>
      <Typography.Paragraph>
        测试设置不会关闭业务渠道；显示页面不会恢复测试。仅隐藏时，原有运行计划仍可能产生费用。
      </Typography.Paragraph>
      <Tabs type='line'>
        <TabPane tab='测试设置' itemKey='settings'>
          <Form
            key={row.revision}
            initValues={row.config}
            onSubmit={(config) =>
              save('save_config', {
                config: { ...row.config, ...config },
              }).catch(() => {})
            }
            disabled={disabled}
          >
            <Form.InputNumber
              field='interval_minutes'
              label='自定义间隔（分钟）'
              min={1}
              max={1440}
            />
            <Form.Input field='timezone' label='时区' />
            <Form.Switch field='all_day' label='全天运行' />
            <Form.InputNumber
              field='daily_budget_micros'
              label='日预算（百万分之一美元）'
              min={0}
            />
            <Form.InputNumber
              field='call_reserve_micros'
              label='每次调用批准费用上界（百万分之一美元）'
              min={0}
            />
            <Form.InputNumber
              field='judge_channel_id'
              label='独立视觉评审渠道 ID'
              min={0}
            />
            <Form.Input field='judge_model' label='视觉评审模型' />
            <Button htmlType='submit' disabled={disabled}>
              保存测试计划
            </Button>
          </Form>
          <Table
            pagination={{ pageSize: 10 }}
            dataSource={targets}
            rowKey={(v) => `${v.group_uid}-${v.channel_id}-${v.model}`}
            columns={[
              { title: '分组', dataIndex: 'group' },
              { title: '渠道', dataIndex: 'channel_name' },
              { title: '模型', dataIndex: 'model' },
              {
                title: '测试状态',
                render: (_, v) => (v.eligible ? '可执行' : v.reason),
              },
              {
                title: '操作',
                render: (_, v) => (
                  <Button disabled={disabled} onClick={() => exclude(v)}>
                    {row.config.excluded.some(
                      (e) =>
                        e.channel_id === v.channel_id &&
                        !e.group_uid &&
                        !e.model,
                    )
                      ? '解除全站停测'
                      : '此渠道全站停测'}
                  </Button>
                ),
              },
            ]}
          />
          {groups.map((g, index) => (
            <Form
              key={`${g.group_uid}-${row.revision}`}
              initValues={
                row.draft.groups[g.group_uid] || {
                  name: '',
                  description_mode: 'inherit',
                  description: '',
                  hidden: false,
                }
              }
              disabled={disabled}
              onSubmit={(value) => {
                const presentation = {
                  ...row.draft,
                  groups: {
                    ...row.draft.groups,
                    [g.group_uid]: { ...value, name: value.name || null },
                  },
                };
                save('save_draft', { presentation }).catch(() => {});
              }}
            >
              <Typography.Title heading={6}>
                {g.routing_key} ·{' '}
                {g.ratio === null ? '倍率待配置' : `×${g.ratio}`}
              </Typography.Title>
              <Space>
                <Button
                  disabled={disabled || index === 0}
                  onClick={() => {
                    const next = [...groups];
                    [next[index], next[index - 1]] = [
                      next[index - 1],
                      next[index],
                    ];
                    save('set_order', { order: next.map((v) => v.group_uid) })
                      .then(() => setGroups(next))
                      .catch(() => {});
                  }}
                >
                  整行上移
                </Button>
                <Button
                  disabled={disabled || index === groups.length - 1}
                  onClick={() => {
                    const next = [...groups];
                    [next[index], next[index + 1]] = [
                      next[index + 1],
                      next[index],
                    ];
                    save('set_order', { order: next.map((v) => v.group_uid) })
                      .then(() => setGroups(next))
                      .catch(() => {});
                  }}
                >
                  整行下移
                </Button>
              </Space>
              <Form.Input
                field='name'
                label='用户端 → 分组卡片 → 显示名称（留空继承）'
                maxLength={40}
              />
              <Form.Select
                field='description_mode'
                label='描述来源'
                optionList={[
                  { label: '继承', value: 'inherit' },
                  { label: '自定义', value: 'custom' },
                  { label: '隐藏', value: 'hidden' },
                ]}
              />
              <Form.TextArea
                field='description'
                label='用户端 → 分组卡片 → 描述'
                maxCount={160}
              />
              <Form.Switch field='hidden' label='仅隐藏此分组（仍参与测试）' />
              <Button htmlType='submit' disabled={disabled}>
                保存展示草稿
              </Button>
            </Form>
          ))}
          <CopyEditor
            key={JSON.stringify(row.draft.copy)}
            saved={row.draft.copy || {}}
            disabled={disabled}
            onSave={(copy) => {
              save('save_draft', {
                presentation: { ...row.draft, copy },
              }).catch(() => {});
            }}
          />
          <Button disabled={disabled} onClick={() => act('publish')}>
            应用展示草稿
          </Button>
        </TabPane>
        <TabPane tab='运行记录' itemKey='runs'>
          <Button onClick={load}>刷新记录</Button>
          <Table
            dataSource={runs}
            rowKey='id'
            pagination={{ pageSize: 20 }}
            columns={[
              { title: '渠道', dataIndex: 'channel_id' },
              { title: '模型', dataIndex: 'model' },
              { title: '状态', dataIndex: 'status' },
              {
                title: '详情',
                render: (_, v) => (
                  <Button
                    onClick={async () => {
                      const { data } = await API.get(
                        `/api/capability/admin/runs/${v.id}`,
                      );
                      if (data.success) setDetail(data.data);
                    }}
                  >
                    调用证据
                  </Button>
                ),
              },
            ]}
          />
        </TabPane>
      </Tabs>
      <SideSheet
        title='实际调用与测试数据'
        visible={!!detail}
        onCancel={() => setDetail(null)}
        width='min(800px, 100vw)'
      >
        <pre style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
          {JSON.stringify(detail, null, 2)}
        </pre>
      </SideSheet>
    </Card>
  );
}
