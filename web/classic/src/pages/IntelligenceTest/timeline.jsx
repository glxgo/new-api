import React, { useEffect, useState } from 'react';
import { Button, Spin, Tooltip } from '@douyinfe/semi-ui';
import { API } from '../../helpers';

const kinds = [
  ['logic', '逻辑'],
  ['geometry', '规则图形'],
  ['scene', '场景绘图'],
];
const state = (item) => {
  if (!item || item.score === null) return ['pending', '待评审'];
  if (item.score === item.maximum) return ['complete', '全部要求已满足'];
  if (item.score === 0) return ['incorrect', '未满足题目要求'];
  return ['partial', '部分要求已满足'];
};
const time = (value) => new Date(value * 1000).toLocaleString();

export default function ResultTimeline({ groupUID, model, revision, onOpen }) {
  const [rounds, setRounds] = useState(null);
  const [failed, setFailed] = useState(false);
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const abort = new AbortController();
    let active = true;
    const refresh = async () => {
      try {
        const { data } = await API.get(
          `/api/capability/groups/${groupUID}/timeline`,
          { params: { model }, signal: abort.signal },
        );
        if (!data.success) throw new Error();
        if (active) {
          setRounds(data.data);
          setFailed(false);
        }
      } catch {
        if (active) setFailed(true);
      }
    };
    refresh();
    const timer = setInterval(refresh, 30000);
    return () => {
      active = false;
      abort.abort();
      clearInterval(timer);
    };
  }, [groupUID, model, revision, retry]);
  if (failed)
    return (
      <p className='iq-classic-muted'>
        暂时无法读取测试历史。
        <Button size='small' onClick={() => setRetry(retry + 1)}>
          重试
        </Button>
      </p>
    );
  if (!rounds) return <Spin />;
  return (
    <section className='iq-classic-timeline'>
      <div className='iq-classic-row'>
        <strong>分项结果与历史</strong>
        <span className='iq-classic-muted'>点击色块查看原题与评定</span>
      </div>
      {!rounds.length ? (
        <p className='iq-classic-muted'>暂无已结束的测试轮次</p>
      ) : (
        <>
          <div className='iq-classic-history-scroll'>
            <div className='iq-classic-history-track'>
              {kinds.map(([kind, label]) => (
                <div key={kind}>
                  <div className='iq-classic-row iq-classic-task-label'>
                    <strong>{label}</strong>
                    <span className='iq-classic-muted'>
                      已评定轮次：
                      {
                        rounds.filter((r) =>
                          r.items.some(
                            (i) => i.kind === kind && i.score !== null,
                          ),
                        ).length
                      }
                      /{rounds.length}
                    </span>
                  </div>
                  <div
                    className='iq-classic-cells'
                    role='group'
                    aria-label={label}
                    style={{
                      gridTemplateColumns: `repeat(${rounds.length}, minmax(0, 1.5rem))`,
                    }}
                  >
                    {rounds.map((round) => {
                      const item = round.items.find((i) => i.kind === kind);
                      const [color, description] = state(item);
                      const tip = `${label} · ${time(round.slot)} · ${description} · ${item?.score ?? '—'}/${item?.maximum ?? '—'} · 已评样本 ${item?.evaluated || 0}/${round.samples} · 方法版本 ${round.suite || '—'}`;
                      return (
                        <Tooltip key={round.slot} content={tip}>
                          <button
                            type='button'
                            aria-label={tip}
                            aria-disabled={!item?.public_id}
                            onClick={() => {
                              if (item?.public_id) onOpen(item.public_id, kind);
                            }}
                            className='iq-classic-cell'
                          >
                            <span
                              className={`iq-classic-cell-fill iq-classic-${color}`}
                            />
                          </button>
                        </Tooltip>
                      );
                    })}
                  </div>
                </div>
              ))}
              <div className='iq-classic-row iq-classic-muted'>
                <span>{time(rounds[0].slot)}</span>
                <span>{time(rounds.at(-1).slot)}</span>
              </div>
            </div>
          </div>
          <div className='iq-classic-legend'>
            {[
              ['complete', '全部要求已满足'],
              ['partial', '部分要求已满足'],
              ['incorrect', '未满足题目要求'],
              ['pending', '待评审'],
            ].map(([color, label]) => (
              <span key={color}>
                <i aria-hidden className={`iq-classic-${color}`} />
                {label}
              </span>
            ))}
          </div>
          <p className='iq-classic-muted'>
            最多显示最近 48
            个已结束轮次。每格对应该轮该题型的精选样本；缺失或待评结果不计为答错。
          </p>
        </>
      )}
    </section>
  );
}
