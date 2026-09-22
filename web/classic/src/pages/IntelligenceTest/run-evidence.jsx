import React, { useEffect, useState } from 'react';
import { SideSheet, Tabs, TabPane, Tag, Spin, Banner } from '@douyinfe/semi-ui';
import { API } from '../../helpers';
import ArtifactPlayer from './artifact-player';

const kinds = { logic: '逻辑', geometry: '规则图形', scene: '场景绘图' };
const statuses = { pass: '已满足', fail: '未满足', uncertain: '无法确认' };
const time = (value) => (value ? new Date(value * 1000).toLocaleString() : '—');

export default function RunEvidence({ selection, groupName, onClose, Image }) {
  const [record, setDetail] = useState(null);
  const detail = record?.public_id === selection?.id ? record : null;
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setDetail(null);
    setFailed(false);
    if (!selection) return;
    const abort = new AbortController();
    API.get(`/api/capability/runs/${selection.id}`, { signal: abort.signal })
      .then(({ data }) => {
        if (!data.success) throw new Error();
        if (!abort.signal.aborted) setDetail(data.data);
      })
      .catch(() => {
        if (!abort.signal.aborted) setFailed(true);
      });
    return () => abort.abort();
  }, [selection]);
  return (
    <SideSheet
      visible={!!selection}
      onCancel={onClose}
      title={`${groupName} · ${detail?.model || '题目与评分依据'}`}
      width='min(1000px, 100vw)'
    >
      {failed ? (
        <Banner type='warning' description='无法读取测试证据，请关闭后重试。' />
      ) : !detail ? (
        <Spin />
      ) : (
        <>
          <p className='iq-classic-muted'>
            题目与评分依据 · {time(detail.time)} · 样本 {detail.sample}
          </p>
          <Tabs
            key={selection.id + selection.kind}
            defaultActiveKey={selection.kind}
          >
            {detail.items.map((item) => {
              const metrics = detail.metrics?.[item.kind];
              const checks =
                item.judgment?.items ||
                item.checks?.map((pass) => ({
                  status: pass ? 'pass' : 'fail',
                  evidence: '',
                })) ||
                [];
              const graded =
                item.status === 'graded' &&
                checks.length > 0 &&
                checks.every((c) => c.status === 'pass' || c.status === 'fail');
              const met = checks.filter((c) => c.status === 'pass').length;
              let status = '待评审';
              if (graded)
                status =
                  met === checks.length ? '全部要求已满足' : '部分要求已满足';
              if (graded && met === 0) status = '未满足题目要求';
              const entries = [
                ['生成开始', time(metrics?.started_at)],
                [
                  '生成耗时',
                  metrics?.duration_seconds == null
                    ? '—'
                    : `${metrics.duration_seconds} 秒`,
                ],
                ['输入 Token', metrics?.input_tokens?.toLocaleString() ?? '—'],
                ['输出 Token', metrics?.output_tokens?.toLocaleString() ?? '—'],
                ['生成调用次数', metrics?.attempts ?? '—'],
                ['方法版本', detail.suite || '—'],
              ];
              return (
                <TabPane
                  key={item.kind}
                  itemKey={item.kind}
                  tab={kinds[item.kind] || item.kind}
                >
                  <article className='iq-classic-evidence'>
                    <div className='iq-classic-row'>
                      <Tag>{status}</Tag>
                      <strong>
                        满足要求：{graded ? `${met} / ${checks.length}` : '—'}
                      </strong>
                    </div>
                    <ArtifactPlayer item={item} Image={Image} />
                    <section>
                      <h3>原始题目</h3>
                      <div
                        className='iq-classic-question'
                        role='region'
                        aria-label='原始题目'
                        tabIndex={0}
                      >
                        {item.question.prompt}
                      </div>
                    </section>
                    <dl className='iq-classic-metrics'>
                      {entries.map(([label, value]) => (
                        <div key={label}>
                          <dt>{label}</dt>
                          <dd>{value}</dd>
                        </div>
                      ))}
                    </dl>
                    <p className='iq-classic-muted'>
                      生成数据不包含渲染与评审；Token
                      数量采用上游报告值，未记录项以“—”表示。
                    </p>
                    {!!checks.length && (
                      <section>
                        <h3>分项评定</h3>
                        {checks.map((check, index) => (
                          <div
                            key={index}
                            style={{
                              borderTop: '1px solid var(--semi-color-border)',
                              paddingBlock: 8,
                            }}
                          >
                            <div className='iq-classic-row'>
                              <strong>
                                {item.question.requirements?.[index] ||
                                  `评定项 ${index + 1}`}
                              </strong>
                              <Tag>{statuses[check.status] || '无法确认'}</Tag>
                            </div>
                            {check.evidence && (
                              <p className='iq-classic-muted'>
                                {check.evidence}
                              </p>
                            )}
                          </div>
                        ))}
                      </section>
                    )}
                    <details open={item.kind === 'logic'}>
                      <summary>原始回答</summary>
                      <pre tabIndex={0}>{item.answer || '暂无回答记录。'}</pre>
                    </details>
                    <p
                      className='iq-classic-muted'
                      style={{ overflowWrap: 'anywhere' }}
                    >
                      记录编号：{detail.public_id}
                      <br />
                      题目指纹：{item.question.hash}
                    </p>
                  </article>
                </TabPane>
              );
            })}
          </Tabs>
        </>
      )}
    </SideSheet>
  );
}
