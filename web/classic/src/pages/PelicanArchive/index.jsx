import React, { useEffect, useState } from 'react';
import {
  Card,
  Button,
  Select,
  Tag,
  Empty,
  Spin,
  SideSheet,
  Tabs,
  TabPane,
  Typography,
  Banner,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { ImageOff } from 'lucide-react';
import { API } from '../../helpers';
import './pelican.css';

export const archiveBase = '/api/pelican-archive';
export const resultLabel = (grade) =>
  ({
    correct: '答案匹配',
    wrong: '答案未匹配',
    no_svg: '未返回作品',
    no_answer: '未检测到答案',
    error: '暂无测试结果',
  })[grade] || '暂无测试结果';
export const time = (value) =>
  value ? new Date(value * 1000).toLocaleString() : '—';
export function useIntelligenceOverview(preview = false) {
  const [data, setData] = useState(null),
    [error, setError] = useState(false);
  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const response = await API.get(
          archiveBase + (preview ? '/admin/preview' : '/groups'),
        );
        if (!response.data.success) throw new Error();
        if (active) {
          setData(response.data);
          setError(false);
        }
      } catch {
        if (active) {
          setError(true);
          setData(null);
        }
      }
    };
    load();
    const interval = setInterval(load, 30000);
    return () => {
      active = false;
      clearInterval(interval);
    };
  }, [preview]);
  return { data, error };
}
function ArtworkUnavailable() {
  const { t } = useTranslation();
  return (
    <div className='pelican-artwork-empty'>
      <Empty
        image={<ImageOff aria-hidden='true' size={28} strokeWidth={1.5} />}
        description={t('作品暂时不可用')}
      />
    </div>
  );
}
function Artwork({ id, admin }) {
  const { t } = useTranslation();
  const [url, setUrl] = useState('');
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setUrl('');
    setFailed(false);
    const abort = new AbortController();
    let local = '';
    API.get(`${archiveBase}${admin ? '/admin' : ''}/records/${id}/artwork`, {
      signal: abort.signal,
      responseType: 'blob',
      disableDuplicate: true,
    })
      .then(({ data }) => {
        if (!abort.signal.aborted && data.type === 'image/svg+xml') {
          local = URL.createObjectURL(data);
          setUrl(local);
        } else if (!abort.signal.aborted) {
          setFailed(true);
        }
      })
      .catch(() => {
        if (!abort.signal.aborted) setFailed(true);
      });
    return () => {
      abort.abort();
      if (local) URL.revokeObjectURL(local);
    };
  }, [id, admin]);
  if (failed) return <ArtworkUnavailable />;
  return url ? (
    <img
      src={url}
      alt={t('模型实际生成作品')}
      className='pelican-artwork'
      onError={() => setFailed(true)}
    />
  ) : (
    <div className='pelican-artwork-empty'>
      <Spin />
    </div>
  );
}
export function Evidence({ id, admin, onClose }) {
  const { t } = useTranslation();
  const [data, setData] = useState(null),
    [error, setError] = useState(false);
  useEffect(() => {
    if (!id) return;
    const abort = new AbortController();
    setData(null);
    setError(false);
    const load = () =>
      API.get(`${archiveBase}${admin ? '/admin' : ''}/records/${id}`, {
        signal: abort.signal,
        disableDuplicate: true,
      })
        .then((r) => {
          if (!abort.signal.aborted) {
            if (!r.data.success) throw new Error();
            setData(r.data.data);
            setError(false);
          }
        })
        .catch(() => {
          if (!abort.signal.aborted) setError(true);
        });
    load();
    const interval = setInterval(load, 30000);
    return () => {
      abort.abort();
      clearInterval(interval);
    };
  }, [id, admin]);
  return (
    <SideSheet
      visible={!!id}
      onCancel={onClose}
      width='min(800px, 100vw)'
      title={t('鹈鹕测试记录')}
    >
      {error ? (
        <Banner type='danger' description={t('无法读取测试记录')} />
      ) : !data ? (
        <Spin />
      ) : (
        <div className='pelican-stack'>
          <div className='pelican-evidence-summary'>
            <strong>{data.model}</strong>
            <Typography.Text type='tertiary'>{t('来源判定')}</Typography.Text>
            <Tag>{t(resultLabel(data.grade))}</Tag>
            {data.truncated && <Tag>{t('作品不完整')}</Tag>}
          </div>
          <Typography.Text type='tertiary'>{time(data.time)}</Typography.Text>
          <Tabs
            key={data.id}
            defaultActiveKey={data.has_artwork ? 'artwork' : 'question'}
          >
            {data.has_artwork && (
              <TabPane tab={t('原始作品')} itemKey='artwork'>
                <ArtworkView key={data.id} id={data.id} admin={admin} />
              </TabPane>
            )}
            <TabPane tab={t('题目与答案')} itemKey='question'>
              <dl className='pelican-answer-grid'>
                <div>
                  <dt>{t('标准答案')}</dt>
                  <dd>{data.expected_answer}</dd>
                </div>
                <div>
                  <dt>{t('检测答案')}</dt>
                  <dd>{data.reported_answer ?? '—'}</dd>
                </div>
              </dl>
              <p className='pelican-evidence-note'>
                {t(
                  '判定以图中是否包含配置答案为准，检测到的单个数字可能与匹配结果不同。',
                )}
              </p>
              <h3>{t('本次记录对应题目')}</h3>
              <p className='pelican-question'>
                {data.prompt || t('本条记录未保存可对应的原题')}
              </p>
              {data.prompt && (
                <p className='pelican-evidence-note'>
                  {t(
                    '题目按版本标记与来源配置匹配；来源未单独存储历史题目全文。',
                  )}
                </p>
              )}
            </TabPane>
            <TabPane tab={t('记录数据')} itemKey='data'>
              <dl className='pelican-record-grid'>
                {[
                  [t('模型'), data.model],
                  [t('测试时间'), time(data.time)],
                  [t('响应耗时'), `${data.latency_ms} ms`],
                  [t('首字耗时'), `${data.ttft_ms} ms`],
                  [t('输入 Token'), data.input_tokens],
                  [t('输出 Token'), data.output_tokens],
                  [t('尝试次数'), data.attempts],
                  [t('题目版本'), data.prompt_hash],
                  [t('记录编号'), data.id],
                ].map(([label, value]) => (
                  <div key={label}>
                    <dt>{label}</dt>
                    <dd>{value}</dd>
                  </div>
                ))}
              </dl>
              <p className='pelican-evidence-note'>
                {t('数据由外部来源报告，零值可能表示未取得用量。')}
              </p>
            </TabPane>
            {admin && (
              <TabPane tab={t('来源原记录')} itemKey='source'>
                <pre className='pelican-source'>
                  {JSON.stringify(data.source_record, null, 2)}
                </pre>
              </TabPane>
            )}
          </Tabs>
        </div>
      )}
    </SideSheet>
  );
}
function ArtworkView({ id, admin }) {
  const { t } = useTranslation();
  const [zoom, setZoom] = useState('fit');
  return (
    <section className='pelican-stack'>
      <div className='pelican-actions'>
        <Typography.Text type='tertiary'>{t('原始作品')}</Typography.Text>
        <Select
          aria-label={t('作品缩放')}
          value={zoom}
          onChange={setZoom}
          optionList={[
            { value: 'fit', label: t('适应宽度') },
            { value: '150', label: '150%' },
            { value: '200', label: '200%' },
          ]}
        />
      </div>
      <div
        className='pelican-artwork-viewport'
        tabIndex={0}
        role='region'
        aria-label={t('原始作品')}
      >
        <div className='pelican-artwork-zoom' data-zoom={zoom}>
          <Artwork id={id} admin={admin} />
        </div>
      </div>
      <p className='pelican-evidence-note'>
        {t('画面、文字与算式均为模型原始输出，缩放仅改变查看尺寸。')}
      </p>
    </section>
  );
}
const chapters = [
  [
    '对象与判定',
    '鹈鹕测试结合 SVG 绘图与数字推理回答。作品和来源判定记录一次实际作答表现，不构成标准化 IQ 分数。',
  ],
  [
    '从题目到证据',
    '外部测试服务负责出题、调用、提取 SVG 并按配置答案判定。本站保存和展示存档，不重复调用或重新评分。',
  ],
  [
    '精选与来源',
    '每个分组按模型最多展示三个渠道的近期结果，优先展示可查看作品中答案匹配的记录。多分组引用同一渠道结果不构成多个独立样本。',
  ],
  [
    '阅读结果',
    '答案匹配表示来源在 SVG 文本中找到配置答案；检测数字是另一个提取结果。无图、未检测到答案和测试中断分别记录，不伪造零分。',
  ],
  [
    '追溯与比较',
    '每个渠道的新完整记录同时替换图像与答案，新记录未完成时保留上一次记录及原时间。各渠道可在不同时间完成，不将其描述成同一轮。',
  ],
  [
    '结果含义',
    '同题目版本的连续记录有助于观察任务表现。精选结果不代表每次请求的平均表现，也不能认证上游身份。分组和倍率沿用本站配置，历史记录保留实际测试时间。',
  ],
];
const titleSlots = [
  'method_scope_title',
  'method_evidence_title',
  'method_selection_title',
  'method_reading_title',
  'method_trace_title',
  'method_limits_title',
];
export default function PelicanArchive({ preview = false }) {
  const { t } = useTranslation();
  const { data, error } = useIntelligenceOverview(preview);
  const [selected, setSelected] = useState(''),
    [chapter, setChapter] = useState(null);
  if (error)
    return <Banner type='danger' description={t('无法读取测试结果')} />;
  if (!data) return <Spin />;
  if (!data.visible) return <Empty description={t('此页面暂不可用')} />;
  const models = [...new Set(data.data.flatMap((g) => g.models))],
    model = models.includes(selected) ? selected : models[0] || '',
    copy = data.presentation.copy;
  return (
    <div className='pelican-stack'>
      <div className='pelican-page-heading'>
        <Typography.Title heading={3}>{t('智商测试')}</Typography.Title>
        <Typography.Text type='tertiary'>
          {t('以原始作品与作答记录，观察模型的任务表现。')}
        </Typography.Text>
      </div>
      <Card className='pelican-hero-card'>
        <div className='pelican-hero-glow' aria-hidden='true' />
        <div className='pelican-hero-rule' aria-hidden='true' />
        <div className='pelican-hero-content'>
          <div className='pelican-eyebrow-row'>
            <span className='pelican-eyebrow pelican-eyebrow-verified'>
              {t('Verified source archive')}
            </span>
            <Typography.Text type='tertiary'>
              {t('Original records preserved')}
            </Typography.Text>
          </div>
          <div className='pelican-hero-title-wrap'>
            <Typography.Title heading={4} className='pelican-hero-title'>
              {copy.page_title || t('鹈鹕测试记录')}
            </Typography.Title>
          </div>
          <Typography.Text type='tertiary' className='pelican-hero-intro'>
            {copy.page_intro ||
              t(
                '查阅渠道测试保存的 SVG 作品和数字答案，点击作品或历史色块查看完整记录。',
              )}
          </Typography.Text>
          <div className='pelican-hero-footer'>
            <div className='pelican-meta-grid'>
              <div className='pelican-meta-chip'>
                <span>{t('存档获取时间')}</span>
                <strong>
                  {data.source_captured_at
                    ? new Date(data.source_captured_at).toLocaleString()
                    : '—'}
                </strong>
              </div>
              <div className='pelican-meta-chip'>
                <span>{t('来源周期')}</span>
                <strong>
                  {data.source_interval_minutes || '—'} {t('分钟')}
                </strong>
              </div>
            </div>
            <label className='pelican-model-field'>
              <span>{t('Model')}</span>
              <Select
                value={model}
                onChange={setSelected}
                optionList={models.map((m) => ({ label: m, value: m }))}
              />
            </label>
          </div>
        </div>
      </Card>
      {data.data
        .filter((g) => g.models.includes(model))
        .map((g) => (
          <Group
            key={g.group_uid + model}
            group={g}
            model={model}
            preview={preview}
            view={data.presentation}
          />
        ))}
      {!models.length && (
        <Empty description={copy.empty_text || t('暂无测试记录')} />
      )}
      {data.presentation.show_method && (
        <Card className='pelican-method-card'>
          <div className='pelican-method-header'>
            <div className='pelican-eyebrow-row'>
              <span className='pelican-eyebrow'>{t('Reading framework')}</span>
              <Typography.Text type='tertiary'>
                {t('Evidence and interpretation')}
              </Typography.Text>
            </div>
            <Typography.Title heading={4}>
              {t('测试方法与结果说明')}
            </Typography.Title>
            <Typography.Text type='tertiary' className='pelican-method-intro'>
              {copy.method_intro || t('了解测试任务、精选规则与证据解释。')}
            </Typography.Text>
          </div>
          <div className='pelican-grid pelican-method-grid'>
            {[0, 2, 5].map((i, index) => (
              <section key={i} className='pelican-method-item'>
                <div className='pelican-method-item-top'>
                  <span className='pelican-method-index'>
                    {String(index + 1).padStart(2, '0')}
                  </span>
                  <span className='pelican-method-label'>{t('Chapter')}</span>
                </div>
                <h4>{copy[titleSlots[i]] || t(chapters[i][0])}</h4>
                <p>{t(chapters[i][1])}</p>
                <Button theme='borderless' onClick={() => setChapter(i)}>
                  {t('阅读详情')}
                </Button>
              </section>
            ))}
          </div>
        </Card>
      )}
      <SideSheet
        visible={chapter !== null}
        onCancel={() => setChapter(null)}
        width='min(680px, 100vw)'
        title={t('测试方法与结果说明')}
      >
        {chapter !== null && (
          <div className='pelican-stack'>
            <Select
              value={chapter}
              onChange={setChapter}
              optionList={chapters.map((v, i) => ({
                label: copy[titleSlots[i]] || t(v[0]),
                value: i,
              }))}
            />
            <Typography.Title heading={4}>
              {copy[titleSlots[chapter]] || t(chapters[chapter][0])}
            </Typography.Title>
            <p>{t(chapters[chapter][1])}</p>
          </div>
        )}
      </SideSheet>
    </div>
  );
}
function Group({ group, model, preview, view }) {
  const { t } = useTranslation();
  const [data, setData] = useState(null),
    [error, setError] = useState(false),
    [detail, setDetail] = useState(null);
  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const r = await API.get(
          `${archiveBase}${preview ? '/admin' : ''}/groups/${group.group_uid}/results?model=${encodeURIComponent(model)}`,
        );
        if (!r.data.success) throw new Error();
        if (active) {
          setData(r.data.data);
          setError(false);
        }
      } catch {
        if (active) setError(true);
      }
    };
    load();
    const timer = setInterval(load, 30000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [group.group_uid, model, preview, view]);
  return (
    <Card className='pelican-group-card'>
      <div className='pelican-group-heading'>
        <div className='pelican-group-name'>
          <span className='pelican-group-eyebrow'>{t('分组结果')}</span>
          <Typography.Title heading={5}>{group.display_name}</Typography.Title>
          <Typography.Text type='tertiary' className='pelican-group-model'>
            {model}
          </Typography.Text>
        </div>
        {group.ratio !== null && (
          <Tag className='pelican-ratio-tag'>{group.ratio}×</Tag>
        )}
      </div>
      {group.description && (
        <p className='pelican-group-description'>{group.description}</p>
      )}
      {error ? (
        <Banner type='danger' description={t('无法读取测试结果')} />
      ) : !data ? (
        <Spin />
      ) : !data.gallery.length ? (
        <Empty
          description={view.copy.empty_text || t('等待已关联渠道的测试记录')}
        />
      ) : (
        <div className='pelican-stack'>
          <div className='pelican-gallery-heading'>
            <p>{view.copy.gallery_title || t('渠道精选记录')}</p>
            <span>
              {t('{{count}} linked targets', { count: data.targets })}
            </span>
          </div>
          <div className='pelican-grid'>
            {data.gallery.map((r) => (
              <button
                key={r.id}
                type='button'
                className='pelican-work'
                onClick={() => setDetail(r.id)}
                aria-label={`${t('查看')} ${time(r.time)}`}
              >
                {r.has_artwork ? (
                  <Artwork key={r.id} id={r.id} admin={preview} />
                ) : (
                  <ArtworkUnavailable />
                )}
              </button>
            ))}
          </div>
          {view.show_history && (
            <div>
              <p>{t('测试历史')}</p>
              <div className='pelican-history-shell'>
                <div className='pelican-history'>
                  {[...data.history].reverse().map((r) => (
                    <button
                      key={r.id}
                      title={`${time(r.time)} · ${t(resultLabel(r.grade))}`}
                      aria-label={`${time(r.time)} · ${t(resultLabel(r.grade))}`}
                      onClick={() => setDetail(r.id)}
                    >
                      <span data-grade={r.grade} />
                    </button>
                  ))}
                </div>
              </div>
              <p className='pelican-history-note'>
                {t('历史为实际保存的记录，各渠道完成时间可能不同。')}
              </p>
            </div>
          )}
        </div>
      )}
      <Evidence id={detail} admin={preview} onClose={() => setDetail(null)} />
    </Card>
  );
}
