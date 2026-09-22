import React, { useEffect, useState } from 'react';
import {
  Card,
  Button,
  Select,
  Tag,
  Empty,
  Spin,
  SideSheet,
  Typography,
  Banner,
} from '@douyinfe/semi-ui';
import { API } from '../../helpers';
import ResultTimeline from './timeline';
import RunEvidence from './run-evidence';
import './intelligence-test.css';

export function useIntelligenceOverview() {
  const [data, setData] = useState(null);
  const [error, setError] = useState(false);
  useEffect(() => {
    let active = true;
    const refresh = async () => {
      try {
        const res = await API.get('/api/capability/groups');
        if (!res.data.success) throw new Error(res.data.message);
        if (active) {
          setData(res.data);
          setError(false);
        }
      } catch {
        if (active) {
          setError(true);
          setData(null);
        }
      }
    };
    refresh();
    const timer = setInterval(refresh, 30000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, []);
  return { data, error };
}

function AuthImage({ src }) {
  const [url, setUrl] = useState('');
  useEffect(() => {
    const abort = new AbortController();
    let local = '';
    setUrl('');
    API.get(src, { responseType: 'blob', signal: abort.signal })
      .then(({ data }) => {
        if (!abort.signal.aborted && data.type === 'image/png') {
          local = URL.createObjectURL(data);
          setUrl(local);
        }
      })
      .catch(() => {});
    return () => {
      abort.abort();
      if (local) URL.revokeObjectURL(local);
    };
  }, [src]);
  return url ? (
    <img
      src={url}
      alt='模型实际生成作品'
      style={{ width: '100%', aspectRatio: '3 / 2', objectFit: 'contain' }}
    />
  ) : (
    <Empty description='作品暂不可用' />
  );
}

const chapters = [
  [
    '对象与评分',
    '按分组和模型观察任务完成表现。文本模型通过 SVG 代码绘图，本套题不等同图片生成模型测试。逻辑两题、规则图形五项、场景绘图六项分别核验，无法判断的结果保留待评状态。',
  ],
  [
    '从题目到证据',
    '每轮冻结同一版本试题。指定渠道完成作答后，系统保留回答并渲染作品。动画采用统一视口与采样窗口，播放记录和连续帧证据来自同一回答；画面变化本身不代表完成题目动作。浏览、刷新和阅读详情不会发起模型调用。',
  ],
  [
    '精选与来源',
    '优先呈现本组任务完成表现较好的作品。依次比较逻辑、规则图形、场景要求，视觉呈现作为同等成绩的补充参考。精选样本不代表全部渠道的平均表现。',
  ],
  [
    '如何阅读结果',
    '本题未通过只描述该题结果。调用中断、无回答和待评审不计为正确或答错。每张作品保留真实时间与来源，历史作品不冒充本轮结果。',
  ],
  [
    '追溯与比较',
    '打开作品可核对原题、回答与判分依据。比较需保持模型、档位和题库版本一致。历史归属不因移组而改写，共享样本不重复计算为独立调用。',
  ],
  [
    '结果的含义',
    '智商测试是本站任务型评测的名称，不是标准化人类 IQ 分数。单次结果不能完整代表模型能力，也不能确认上游真实模型身份。视觉判断存在不确定性，须经过校准。',
  ],
];
const chapterKeys = [
  'method_scope_title',
  'method_evidence_title',
  'method_selection_title',
  'method_reading_title',
  'method_trace_title',
  'method_limits_title',
];

export default function IntelligenceTest() {
  const { data, error } = useIntelligenceOverview();
  const [method, setMethod] = useState(false);
  const [chapter, setChapter] = useState(0);
  if (error)
    return (
      <Banner type='warning' description='暂时无法读取测试结果，请稍后重试。' />
    );
  if (!data) return <Spin />;
  if (!data.visible) return <Empty description='此页面暂未开放' />;
  return (
    <div className='iq-classic-page p-4'>
      <Card
        title='智商测试'
        headerExtraContent={<Tag>{data.running ? '按计划运行' : '已暂停'}</Tag>}
      >
        <Typography.Title heading={5}>
          {data.copy?.page_title || '测试概览'}
        </Typography.Title>
        <Typography.Paragraph>
          {data.running
            ? data.copy?.page_intro ||
              '通过逻辑推理与绘图任务观察模型表现，精选作品可查看原题与评分依据。'
            : '测试计划已暂停，已完成的结果仍可查阅。'}
        </Typography.Paragraph>
        {data.show_method && (
          <Button onClick={() => setMethod(true)}>测试方法与评分说明</Button>
        )}
      </Card>
      {data.data.map((group) => (
        <GroupResults
          key={group.group_uid}
          group={group}
          count={data.gallery_size}
          revision={data.revision}
          copy={data.copy || {}}
          showHistory={data.show_history}
        />
      ))}
      {!data.data.length && (
        <Empty description={data.copy?.empty_text || '暂无可展示的测试结果'} />
      )}
      {data.show_method && (
        <Card title='测试方法与评分说明'>
          <div className='iq-classic-method-grid'>
            {[0, 2, 5].map((index) => (
              <section key={index}>
                <strong>
                  {data.copy?.[chapterKeys[index]] || chapters[index][0]}
                </strong>
                <p className='iq-classic-muted'>{chapters[index][1]}</p>
                <Button
                  size='small'
                  theme='borderless'
                  onClick={() => {
                    setChapter(index);
                    setMethod(true);
                  }}
                >
                  查看详情
                </Button>
              </section>
            ))}
          </div>
        </Card>
      )}
      <SideSheet
        visible={method && data.show_method}
        onCancel={() => setMethod(false)}
        title='测试方法与评分说明'
        width='min(720px, 100vw)'
      >
        {data.copy?.method_intro && (
          <Typography.Paragraph>{data.copy.method_intro}</Typography.Paragraph>
        )}
        <Select
          value={chapter}
          onChange={setChapter}
          optionList={chapters.map(([label], value) => ({
            label: data.copy?.[chapterKeys[value]] || label,
            value,
          }))}
          style={{ width: '100%' }}
        />
        <Typography.Title heading={4}>
          {data.copy?.[chapterKeys[chapter]] || chapters[chapter][0]}
        </Typography.Title>
        <Typography.Paragraph>{chapters[chapter][1]}</Typography.Paragraph>
        <Button
          disabled={chapter === 0}
          onClick={() => setChapter(chapter - 1)}
        >
          上一节
        </Button>{' '}
        <Button
          disabled={chapter === 5}
          onClick={() => setChapter(chapter + 1)}
        >
          下一节
        </Button>
      </SideSheet>
    </div>
  );
}

function GroupResults({ group, count, revision, copy, showHistory }) {
  const [runs, setRuns] = useState([]);
  const [kind, setKind] = useState('scene');
  const [selection, setSelection] = useState(null);
  useEffect(() => {
    let active = true;
    const refresh = async () => {
      try {
        const { data } = await API.get(
          `/api/capability/groups/${group.group_uid}/results`,
        );
        if (active && data.success) setRuns(data.data);
      } catch {
        if (active) setRuns([]);
      }
    };
    refresh();
    const timer = setInterval(refresh, 30000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [group.group_uid, revision]);
  const open = (id, task = kind) => setSelection({ id, kind: task });
  return (
    <Card
      title={group.display_name}
      headerExtraContent={
        <Tag>{group.ratio === null ? '倍率待配置' : `×${group.ratio}`}</Tag>
      }
    >
      <Typography.Paragraph>{group.description}</Typography.Paragraph>
      {copy.gallery_title && (
        <Typography.Paragraph>{copy.gallery_title}</Typography.Paragraph>
      )}
      <Select
        value={kind}
        onChange={setKind}
        optionList={[
          { label: '场景绘图', value: 'scene' },
          { label: '规则图形', value: 'geometry' },
        ]}
      />
      {group.models.map((model) => {
        const current = runs.filter((r) => r.model === model);
        const latest = Math.max(0, ...current.map((r) => r.slot));
        const all = current.filter((r) => r.slot === latest);
        const works = all
          .filter((r) => r.items.some((i) => i.kind === kind && i.artifact))
          .slice(0, count);
        return (
          <section key={model} style={{ marginTop: 12 }}>
            <div className='iq-classic-row'>
              <strong>{model}</strong>
              <span className='iq-classic-muted'>
                测试轮次：
                {latest ? new Date(latest * 1000).toLocaleString() : '—'}
              </span>
            </div>
            <div className='iq-classic-gallery'>
              {works.map((run) => (
                <Card key={run.public_id} bodyStyle={{ padding: 12 }}>
                  <Button
                    theme='borderless'
                    style={{ height: 'auto', width: '100%' }}
                    onClick={() => open(run.public_id)}
                  >
                    <AuthImage
                      src={run.items.find((i) => i.kind === kind).artifact}
                    />
                  </Button>
                  <Typography.Text size='small'>
                    {new Date(run.time * 1000).toLocaleString()} · 样本{' '}
                    {run.sample}
                  </Typography.Text>
                </Card>
              ))}
            </div>
            {!works.length && (
              <Empty description={copy.empty_text || '暂无已完成作品'} />
            )}
            <Typography.Paragraph style={{ marginTop: 10, fontSize: 12 }}>
              最近轮次样本：{all.length} · 作品保留真实题目与测试时间
            </Typography.Paragraph>
            {showHistory && (
              <ResultTimeline
                groupUID={group.group_uid}
                model={model}
                revision={revision}
                onOpen={open}
              />
            )}
          </section>
        );
      })}
      <RunEvidence
        selection={selection}
        groupName={group.display_name}
        onClose={() => setSelection(null)}
        Image={AuthImage}
      />
    </Card>
  );
}
