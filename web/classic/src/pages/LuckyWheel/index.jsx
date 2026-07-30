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
  Card,
  Col,
  Modal,
  Row,
  Select,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';

const prizeNames = {
  quota_5: '$5 套餐额度',
  quota_10: '$10 套餐额度',
  quota_20: '$20 套餐额度',
  quota_30: '$30 套餐额度',
  quota_50: '$50 套餐额度',
  quota_100: '$100 套餐额度',
  gift_5: '$5 钱包赠金',
  gift_10: '$10 钱包赠金',
  gift_20: '$20 钱包赠金',
  subscription_double: '套餐双倍卡',
  subscription_full_reset: '套餐全额重置卡',
  crazy_5h: '5 小时狂蹬卡',
};

const sourceNames = {
  recharge_threshold: '累计充值',
  subscription_purchase: '购买套餐',
  subscription_reset: '套餐周期重置',
  admin_compensation: '人工补偿',
};

const formatTime = (value) =>
  value ? new Date(value * 1000).toLocaleString() : '—';

const buildWheelSegments = (pool, poolType, rechargeBonusUsdMicros) =>
  pool.map((prize) => {
    if (prize.code.startsWith('quota_')) {
      const actualUsdMicros =
        prize.display_usd_micros +
        (poolType === 'recharge' ? rechargeBonusUsdMicros : 0);
      return {
        ...prize,
        label: `$${(actualUsdMicros / 1000000).toLocaleString('en-US', {
          maximumFractionDigits: 2,
        })} 套餐额度`,
      };
    }
    return { ...prize, label: prizeNames[prize.code] || prize.code };
  });

const getTargetRotation = (currentRotation, prizeIndex, segmentCount) => {
  const count = Math.max(segmentCount, 1);
  const normalizedCurrent = ((currentRotation % 360) + 360) % 360;
  const target = (((360 - (prizeIndex * 360) / count) % 360) + 360) % 360;
  const finalDelta = (target - normalizedCurrent + 360) % 360;
  return currentRotation + 6 * 360 + finalDelta;
};

const getWheelBackground = (segmentCount) => {
  const count = Math.max(segmentCount, 1);
  const slice = 360 / count;
  const stops = Array.from({ length: count }, (_, index) => {
    const color = index % 2 === 0 ? '#fff9ed' : '#f5bd72';
    return `${color} ${index * slice}deg ${(index + 1) * slice}deg`;
  });
  return `conic-gradient(from -${slice / 2}deg,${stops.join(',')})`;
};

const getReadableLabelRotation = (angle) =>
  angle > 90 && angle < 270 ? 180 : 0;

const formatPrizeProbability = (weight) => {
  const probability = weight / 10000;
  if (probability < 1) return probability.toFixed(3);
  if (Number.isInteger(probability)) return probability.toFixed(0);
  return probability.toFixed(3).replace(/0+$/, '').replace(/\.$/, '');
};

const LuckyWheel = () => {
  const [status, setStatus] = useState(null);
  const [cards, setCards] = useState([]);
  const [draws, setDraws] = useState([]);
  const [rules, setRules] = useState([]);
  const [cardId, setCardId] = useState();
  const [drawing, setDrawing] = useState(false);
  const [rotation, setRotation] = useState(0);
  const [pendingResult, setPendingResult] = useState(null);
  const [pendingNextCardId, setPendingNextCardId] = useState();
  const [result, setResult] = useState(null);
  const [rulesVisible, setRulesVisible] = useState(false);

  const load = async (preferredCardId) => {
    const [statusRes, cardsRes, drawsRes, rulesRes] = await Promise.all([
      API.get('/api/lucky-wheel/status'),
      API.get('/api/lucky-wheel/cards?page=1&page_size=100'),
      API.get('/api/lucky-wheel/draws?page=1&page_size=30'),
      API.get('/api/lucky-wheel/rules'),
    ]);
    if (statusRes.data.success) setStatus(statusRes.data.data);
    if (cardsRes.data.success) setCards(cardsRes.data.data.items || []);
    if (drawsRes.data.success) setDraws(drawsRes.data.data.items || []);
    if (rulesRes.data.success) setRules(rulesRes.data.data || []);
    const available = (cardsRes.data.data?.items || [])
      .filter((card) => card.status === 'available')
      .sort((a, b) => a.expires_at - b.expires_at || a.id - b.id);
    setCardId((current) =>
      available.some((card) => card.id === (preferredCardId || current))
        ? preferredCardId || current
        : available[0]?.id,
    );
  };

  useEffect(() => {
    load().catch((error) => showError(error.message));
  }, []);

  const availableCards = useMemo(
    () =>
      cards
        .filter((card) => card.status === 'available')
        .sort((a, b) => a.expires_at - b.expires_at),
    [cards],
  );
  const selectedCard = availableCards.find((card) => card.id === cardId);
  const selectedRule =
    rules.find((rule) => rule.id === selectedCard?.rule_set_id) || rules[0];
  let visiblePool = [];
  try {
    visiblePool = JSON.parse(
      selectedCard?.pool_type === 'recharge'
        ? selectedRule?.recharge_pool || '[]'
        : selectedRule?.subscription_pool || '[]',
    );
  } catch {
    visiblePool = [];
  }
  const wheelSegments = buildWheelSegments(
    visiblePool,
    selectedCard?.pool_type || 'subscription',
    selectedRule?.recharge_bonus_usd_micros || 0,
  );
  const wheelLabelByCode = new Map(
    wheelSegments.map((prize) => [prize.code, prize.label]),
  );

  const draw = async () => {
    if (!cardId || drawing || status?.campaign?.draw_paused) return;
    setDrawing(true);
    try {
      const response = await API.post('/api/lucky-wheel/draws', {
        card_id: cardId,
        idempotency_key:
          globalThis.crypto?.randomUUID?.() ||
          `classic-${Date.now()}-${Math.random().toString(16).slice(2)}`,
      });
      if (!response.data.success) {
        setDrawing(false);
        return;
      }
      const prizeIndex = wheelSegments.findIndex(
        (prize) => prize.code === response.data.data.prize_type,
      );
      if (prizeIndex < 0) {
        throw new Error('抽奖结果不在当前幸运卡奖池中');
      }
      setPendingResult(response.data.data);
      const currentIndex = availableCards.findIndex(
        (card) => card.id === cardId,
      );
      setPendingNextCardId(
        availableCards.length > 1
          ? availableCards[(currentIndex + 1) % availableCards.length]?.id
          : undefined,
      );
      setRotation((value) =>
        getTargetRotation(value, prizeIndex, wheelSegments.length),
      );
    } catch (error) {
      showError(error.message);
      setDrawing(false);
    }
  };

  const finishDrawReveal = async () => {
    if (!pendingResult) return;
    const revealedDraw = pendingResult;
    const nextCardId = pendingNextCardId;
    setPendingResult(null);
    setPendingNextCardId(undefined);
    setDrawing(false);
    try {
      await load(nextCardId);
      showSuccess('奖励已经发放，下一张幸运卡已自动选中');
    } catch (error) {
      showError(`奖励已发放，但幸运卡列表刷新失败：${error.message}`);
    }
    setResult(revealedDraw);
  };

  const columns = [
    { title: '时间', dataIndex: 'awarded_at', render: formatTime },
    { title: '幸运卡', dataIndex: 'card_id', render: (value) => `#${value}` },
    {
      title: '奖品',
      render: (_, record) =>
        record.prize_type.startsWith('quota_')
          ? `$${record.actual_usd_micros / 1000000} 套餐额度`
          : prizeNames[record.prize_type] || record.prize_type,
    },
    {
      title: '状态',
      render: () => <Tag color='green'>已发放</Tag>,
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Typography.Title heading={2} style={{ marginBottom: 20 }}>
        幸运大转盘
      </Typography.Title>
      {status?.campaign?.draw_paused && (
        <Banner
          type='warning'
          description='抽奖暂时暂停，已有幸运卡的有效期正在冻结，恢复后会自动顺延。'
          style={{ marginBottom: 16 }}
        />
      )}
      <Row gutter={16}>
        <Col xs={24} xl={16}>
          <Card
            style={{
              minHeight: 610,
              borderRadius: 24,
              background:
                'radial-gradient(circle at 20% 10%, #fff8ef, #ffd9aa 58%, #f6a35b)',
              overflow: 'hidden',
            }}
          >
            <div style={{ textAlign: 'center', padding: '16px 0' }}>
              <Typography.Title heading={3}>
                转动好运，赢取专属权益
              </Typography.Title>
              <Typography.Text type='tertiary'>
                抽奖结果由服务端安全随机产生
              </Typography.Text>
              <div
                style={{
                  width: 360,
                  height: 360,
                  maxWidth: '86vw',
                  maxHeight: '86vw',
                  margin: '28px auto',
                  position: 'relative',
                }}
              >
                <div
                  style={{
                    position: 'absolute',
                    zIndex: 3,
                    top: -8,
                    left: '50%',
                    width: 0,
                    height: 0,
                    transform: 'translateX(-50%)',
                    borderLeft: '14px solid transparent',
                    borderRight: '14px solid transparent',
                    borderTop: '28px solid #7b2413',
                  }}
                />
                <div
                  onTransitionEnd={(event) => {
                    if (
                      event.currentTarget === event.target &&
                      event.propertyName === 'transform'
                    ) {
                      finishDrawReveal();
                    }
                  }}
                  style={{
                    position: 'absolute',
                    inset: 0,
                    borderRadius: '50%',
                    border: '13px solid #c84020',
                    background: getWheelBackground(wheelSegments.length),
                    boxShadow: '0 24px 45px rgba(139,48,18,.28)',
                    transition: 'transform 4.8s cubic-bezier(.08,.72,.08,1)',
                    transform: `rotate(${rotation}deg)`,
                  }}
                >
                  {wheelSegments.map((prize, index) => {
                    const angle = (360 / wheelSegments.length) * index;
                    return (
                      <div
                        key={prize.code}
                        style={{
                          position: 'absolute',
                          inset: 0,
                          pointerEvents: 'none',
                          transform: `rotate(${angle}deg)`,
                        }}
                      >
                        <span
                          style={{
                            position: 'absolute',
                            top: '9%',
                            left: '50%',
                            width: 104,
                            color: '#6f2b18',
                            fontSize: 12,
                            fontWeight: 650,
                            lineHeight: 1.15,
                            textAlign: 'center',
                            transform: `translateX(-50%) rotate(${getReadableLabelRotation(
                              angle + rotation,
                            )}deg)`,
                          }}
                        >
                          {prize.label}
                        </span>
                      </div>
                    );
                  })}
                </div>
                <button
                  type='button'
                  aria-label={drawing ? '好运正在揭晓' : '点击好运开始抽奖'}
                  title={drawing ? '好运正在揭晓' : '点击中心也可以开始抽奖'}
                  disabled={!cardId || status?.campaign?.draw_paused || drawing}
                  onClick={draw}
                  style={{
                    position: 'absolute',
                    top: '50%',
                    left: '50%',
                    width: '32%',
                    height: '32%',
                    borderRadius: '50%',
                    background: '#df5b34',
                    border: '7px solid #ffd49b',
                    color: '#fff',
                    display: 'grid',
                    placeItems: 'center',
                    fontSize: 20,
                    fontWeight: 700,
                    boxShadow: 'inset 0 2px 9px rgba(120,30,10,.24)',
                    transform: 'translate(-50%, -50%)',
                    zIndex: 2,
                    cursor:
                      !cardId || status?.campaign?.draw_paused || drawing
                        ? 'not-allowed'
                        : 'pointer',
                  }}
                >
                  {drawing ? '揭晓中' : '好运'}
                </button>
              </div>
              <Select
                value={cardId}
                placeholder='选择一张幸运卡'
                style={{ width: 360, maxWidth: '70%' }}
                onChange={setCardId}
                optionList={availableCards.map((card) => ({
                  value: card.id,
                  label: `#${card.id} · ${
                    sourceNames[card.source_type] || card.source_type
                  } · ${formatTime(card.expires_at)} 到期`,
                }))}
              />
              <Button
                theme='solid'
                type='danger'
                size='large'
                loading={drawing}
                disabled={!cardId || status?.campaign?.draw_paused || drawing}
                onClick={draw}
                style={{ marginLeft: 12 }}
              >
                立即抽奖
              </Button>
              <Button
                theme='borderless'
                onClick={() => setRulesVisible(true)}
                style={{ display: 'block', margin: '12px auto 0' }}
              >
                查看概率与规则
              </Button>
            </div>
          </Card>
        </Col>
        <Col xs={24} xl={8}>
          <Card title='我的幸运卡' style={{ marginBottom: 16 }}>
            <Typography.Title heading={1}>
              {status?.available_cards || 0}
            </Typography.Title>
            <Typography.Text type='tertiary'>
              默认选择最早到期的一张
            </Typography.Text>
          </Card>
          <Card title='距离下一张幸运卡'>
            <Typography.Title heading={4}>
              ¥
              {((status?.recharge_progress?.eligible_cents || 0) / 100).toFixed(
                2,
              )}
              {' / '}¥
              {(
                (status?.recharge_progress?.next_threshold_cents || 5000) / 100
              ).toFixed(2)}
            </Typography.Title>
            <Typography.Text type='tertiary'>
              跨过多个充值档位时会一次获得全部对应幸运卡。
            </Typography.Text>
          </Card>
        </Col>
      </Row>
      <Card title='抽奖记录' style={{ marginTop: 16 }}>
        <Table
          columns={columns}
          dataSource={draws}
          rowKey='id'
          pagination={false}
          empty='还没有抽奖记录'
        />
      </Card>
      <Modal
        visible={Boolean(result)}
        onCancel={() => setResult(null)}
        footer={null}
        title='恭喜中奖'
      >
        <Typography.Title heading={3} style={{ textAlign: 'center' }}>
          {result?.prize_type?.startsWith('quota_')
            ? `$${result.actual_usd_micros / 1000000} 套餐额度`
            : prizeNames[result?.prize_type] || result?.prize_type}
        </Typography.Title>
      </Modal>
      <Modal
        visible={rulesVisible}
        onCancel={() => setRulesVisible(false)}
        footer={null}
        title='活动规则与奖池概率'
      >
        {visiblePool.map((prize) => (
          <div
            key={prize.code}
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              padding: '9px 0',
              borderBottom: '1px solid var(--semi-color-border)',
            }}
          >
            <span>
              {wheelLabelByCode.get(prize.code) ||
                prizeNames[prize.code] ||
                prize.code}
            </span>
            <strong>{formatPrizeProbability(prize.weight)}%</strong>
          </div>
        ))}
        <Typography.Paragraph type='tertiary' style={{ marginTop: 16 }}>
          套餐来源奖励跟随来源套餐分组与剩余有效期；充值来源套餐额度会在显示面额上额外增加
          $60。钱包赠金永久有效，但不能用于购买订阅套餐。
        </Typography.Paragraph>
      </Modal>
    </div>
  );
};

export default LuckyWheel;
