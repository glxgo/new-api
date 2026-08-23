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

import React, { useMemo, useState } from 'react';
import {
  Badge,
  Button,
  Card,
  Divider,
  Modal,
  Select,
  Skeleton,
  Space,
  Spin,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess, renderQuota } from '../../helpers';
import { getCurrencyConfig } from '../../helpers/render';
import {
  ArrowDown,
  ArrowUp,
  KeyRound,
  ListOrdered,
  RefreshCw,
  Sparkles,
} from 'lucide-react';
import SubscriptionPurchaseModal from './modals/SubscriptionPurchaseModal';
import {
  formatSubscriptionDuration,
  formatSubscriptionResetPeriod,
} from '../../helpers/subscriptionFormat';

const { Text } = Typography;

// 过滤易支付方式
function getEpayMethods(payMethods = []) {
  return (payMethods || []).filter(
    (m) => m?.type && m.type !== 'stripe' && m.type !== 'creem',
  );
}

// 提交易支付表单
function submitEpayForm({ url, params }) {
  const form = document.createElement('form');
  form.action = url;
  form.method = 'POST';
  const isSafari =
    navigator.userAgent.indexOf('Safari') > -1 &&
    navigator.userAgent.indexOf('Chrome') < 1;
  if (!isSafari) form.target = '_blank';
  Object.keys(params || {}).forEach((key) => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = key;
    input.value = params[key];
    form.appendChild(input);
  });
  document.body.appendChild(form);
  form.submit();
  document.body.removeChild(form);
}

const SubscriptionPlansCard = ({
  t,
  loading = false,
  plans = [],
  payMethods = [],
  enableOnlineTopUp = false,
  enableStripeTopUp = false,
  enableCreemTopUp = false,
  billingPreference,
  onChangeBillingPreference,
  activeSubscriptions = [],
  allSubscriptions = [],
  reloadSubscriptionSelf,
  withCard = true,
}) => {
  const [open, setOpen] = useState(false);
  const [selectedPlan, setSelectedPlan] = useState(null);
  const [paying, setPaying] = useState(false);
  const [selectedEpayMethod, setSelectedEpayMethod] = useState('');
  const [refreshing, setRefreshing] = useState(false);
  const [orderOpen, setOrderOpen] = useState(false);
  const [orderGroup, setOrderGroup] = useState('default');
  const [orderRevision, setOrderRevision] = useState(0);
  const [orderItems, setOrderItems] = useState([]);
  const [orderLoading, setOrderLoading] = useState(false);
  const [orderSaving, setOrderSaving] = useState(false);
  const [manageSubscription, setManageSubscription] = useState(null);
  const [manageHiding, setManageHiding] = useState(false);
  const [restoringId, setRestoringId] = useState(null);

  const epayMethods = useMemo(() => getEpayMethods(payMethods), [payMethods]);

  const openBuy = (p) => {
    setSelectedPlan(p);
    setSelectedEpayMethod(epayMethods?.[0]?.type || '');
    setOpen(true);
  };

  const closeBuy = () => {
    setOpen(false);
    setSelectedPlan(null);
    setPaying(false);
  };

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      await reloadSubscriptionSelf?.();
    } finally {
      setRefreshing(false);
    }
  };

  const payStripe = async () => {
    if (!selectedPlan?.plan?.stripe_price_id) {
      showError(t('该套餐未配置 Stripe'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/stripe/pay', {
        plan_id: selectedPlan.plan.id,
      });
      if (res.data?.message === 'success') {
        window.open(res.data.data?.pay_link, '_blank');
        showSuccess(t('已打开支付页面'));
        closeBuy();
      } else {
        const errorMsg =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('支付失败');
        showError(errorMsg);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const payCreem = async () => {
    if (!selectedPlan?.plan?.creem_product_id) {
      showError(t('该套餐未配置 Creem'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/creem/pay', {
        plan_id: selectedPlan.plan.id,
      });
      if (res.data?.message === 'success') {
        window.open(res.data.data?.checkout_url, '_blank');
        showSuccess(t('已打开支付页面'));
        closeBuy();
      } else {
        const errorMsg =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('支付失败');
        showError(errorMsg);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const payEpay = async () => {
    if (!selectedEpayMethod) {
      showError(t('请选择支付方式'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/epay/pay', {
        plan_id: selectedPlan.plan.id,
        payment_method: selectedEpayMethod,
      });
      if (res.data?.message === 'success') {
        submitEpayForm({ url: res.data.url, params: res.data.data });
        showSuccess(t('已发起支付'));
        closeBuy();
      } else {
        const errorMsg =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('支付失败');
        showError(errorMsg);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  // 当前订阅信息 - 支持多个订阅
  const hasActiveSubscription = activeSubscriptions.length > 0;
  const hasAnySubscription = allSubscriptions.length > 0;
  const disableSubscriptionPreference = !hasActiveSubscription;
  const isSubscriptionPreference =
    billingPreference === 'subscription_first' ||
    billingPreference === 'subscription_only';
  const displayBillingPreference =
    disableSubscriptionPreference && isSubscriptionPreference
      ? 'wallet_first'
      : billingPreference;
  const subscriptionPreferenceLabel =
    billingPreference === 'subscription_only' ? t('仅用订阅') : t('优先订阅');

  const planPurchaseCountMap = useMemo(() => {
    const map = new Map();
    (allSubscriptions || []).forEach((sub) => {
      const planId = sub?.subscription?.plan_id;
      if (!planId) return;
      map.set(planId, (map.get(planId) || 0) + 1);
    });
    return map;
  }, [allSubscriptions]);

  const planTitleMap = useMemo(() => {
    const map = new Map();
    (plans || []).forEach((p) => {
      const plan = p?.plan;
      if (!plan?.id) return;
      map.set(plan.id, plan.title || '');
    });
    return map;
  }, [plans]);

  const subscriptionGroups = useMemo(() => {
    const groups = new Set(['default']);
    (activeSubscriptions || []).forEach((item) => {
      const group = item?.subscription?.allowed_group;
      if (group) groups.add(group);
    });
    return Array.from(groups);
  }, [activeSubscriptions]);

  const loadConsumptionOrder = async (group = orderGroup) => {
    setOrderLoading(true);
    try {
      const res = await API.get(
        `/api/subscription/self/consumption-order?group=${encodeURIComponent(group)}`,
      );
      if (!res.data?.success) {
        showError(res.data?.message || t('读取套餐顺序失败'));
        return;
      }
      const data = res.data.data || {};
      const rank = new Map(
        (data.order || []).map((item) => [
          Number(item.subscription_id),
          Number(item.priority),
        ]),
      );
      const items = (data.subscriptions || [])
        .map((subscription) => ({
          id: Number(subscription.id),
          title:
            subscription.plan_title ||
            planTitleMap.get(subscription.plan_id) ||
            `${t('订阅实例')} #${subscription.id}`,
          endTime: Number(subscription.end_time || 0),
        }))
        .sort((a, b) => {
          const left = rank.get(a.id);
          const right = rank.get(b.id);
          if (left !== undefined && right === undefined) return -1;
          if (left === undefined && right !== undefined) return 1;
          if (left !== undefined && right !== undefined && left !== right) {
            return left - right;
          }
          return a.endTime - b.endTime || a.id - b.id;
        });
      setOrderRevision(Number(data.revision || 0));
      setOrderItems(items);
    } catch (error) {
      showError(t('读取套餐顺序失败'));
    } finally {
      setOrderLoading(false);
    }
  };

  const openConsumptionOrder = () => {
    const group = subscriptionGroups[0] || 'default';
    setOrderGroup(group);
    setOrderOpen(true);
    loadConsumptionOrder(group);
  };

  const moveConsumptionItem = (index, direction) => {
    const target = index + direction;
    if (target < 0 || target >= orderItems.length) return;
    setOrderItems((current) => {
      const next = [...current];
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
  };

  const saveConsumptionOrder = async () => {
    setOrderSaving(true);
    try {
      const res = await API.put('/api/subscription/self/consumption-order', {
        group: orderGroup,
        revision: orderRevision,
        subscription_ids: orderItems.map((item) => item.id),
      });
      if (!res.data?.success) {
        showError(res.data?.message || t('保存失败，请刷新后重试'));
        await loadConsumptionOrder(orderGroup);
        return;
      }
      showSuccess(t('套餐消耗顺序已保存'));
      setOrderRevision(Number(res.data.data?.revision || orderRevision + 1));
      setOrderOpen(false);
    } catch (error) {
      showError(t('保存失败，请刷新后重试'));
      await loadConsumptionOrder(orderGroup);
    } finally {
      setOrderSaving(false);
    }
  };

  const getPlanPurchaseCount = (planId) =>
    planPurchaseCountMap.get(planId) || 0;

  // 计算单个订阅的剩余天数
  const getRemainingDays = (sub) => {
    if (!sub?.subscription?.end_time) return 0;
    const now = Date.now() / 1000;
    const remaining = sub.subscription.end_time - now;
    return Math.max(0, Math.ceil(remaining / 86400));
  };

  // 计算单个订阅的使用进度
  const getUsagePercent = (sub) => {
    const total = Number(sub?.subscription?.amount_total || 0);
    const used = Number(sub?.subscription?.amount_used || 0);
    if (total <= 0) return 0;
    return Math.round((used / total) * 100);
  };

  const hideSubscription = async (subscriptionId) => {
    try {
      const res = await API.patch(
        `/api/subscription/self/instances/${subscriptionId}/visibility`,
        { hidden: true },
      );
      if (res.data?.success) {
        showSuccess('已隐藏该套餐，可联系管理员恢复展示');
        await reloadSubscriptionSelf?.();
      } else {
        showError(res.data?.message || '隐藏失败');
      }
    } catch (e) {
      showError('隐藏失败，请稍后重试');
    }
  };

  const hideFromManage = async () => {
    if (!manageSubscription?.id) return;
    setManageHiding(true);
    try {
      await hideSubscription(manageSubscription.id);
      setManageSubscription(null);
    } finally {
      setManageHiding(false);
    }
  };

  const restoreSubscription = async (subscriptionId) => {
    setRestoringId(subscriptionId);
    try {
      const res = await API.patch(
        `/api/subscription/self/instances/${subscriptionId}/visibility`,
        { hidden: false },
      );
      if (res.data?.success) {
        showSuccess('已恢复展示');
        await reloadSubscriptionSelf?.();
      } else {
        showError(res.data?.message || '恢复展示失败');
      }
    } catch (e) {
      showError('恢复展示失败，请稍后重试');
    } finally {
      setRestoringId(null);
    }
  };

  const cardContent = (
    <>
      {/* 卡片头部 */}
      {loading ? (
        <div className='space-y-4'>
          {/* 我的订阅骨架屏 */}
          <Card className='!rounded-xl w-full' bodyStyle={{ padding: '12px' }}>
            <div className='flex items-center justify-between mb-3'>
              <Skeleton.Title active style={{ width: 100, height: 20 }} />
              <Skeleton.Button active style={{ width: 24, height: 24 }} />
            </div>
            <div className='space-y-2'>
              <Skeleton.Paragraph active rows={2} />
            </div>
          </Card>
          {/* 套餐列表骨架屏 */}
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 2xl:gap-5 w-full px-1'>
            {[1, 2, 3, 4].map((i) => (
              <Card
                key={i}
                className='!rounded-xl w-full h-full'
                bodyStyle={{ padding: 16 }}
              >
                <Skeleton.Title
                  active
                  style={{ width: '60%', height: 24, marginBottom: 8 }}
                />
                <Skeleton.Paragraph
                  active
                  rows={1}
                  style={{ marginBottom: 12 }}
                />
                <div className='text-center py-4'>
                  <Skeleton.Title
                    active
                    style={{ width: '40%', height: 32, margin: '0 auto' }}
                  />
                </div>
                <Skeleton.Paragraph active rows={3} style={{ marginTop: 12 }} />
                <Skeleton.Button
                  active
                  block
                  style={{ marginTop: 16, height: 32 }}
                />
              </Card>
            ))}
          </div>
        </div>
      ) : (
        <Space vertical style={{ width: '100%' }} spacing={8}>
          {/* 当前订阅状态 */}
          <Card className='!rounded-xl w-full' bodyStyle={{ padding: '12px' }}>
            <div className='flex items-center justify-between mb-2 gap-3'>
              <div className='flex items-center gap-2 flex-1 min-w-0'>
                <Text strong>{t('我的订阅')}</Text>
                {hasActiveSubscription ? (
                  <Tag
                    color='white'
                    size='small'
                    shape='circle'
                    prefixIcon={<Badge dot type='success' />}
                  >
                    {activeSubscriptions.length} {t('个生效中')}
                  </Tag>
                ) : (
                  <Tag color='white' size='small' shape='circle'>
                    {t('无生效')}
                  </Tag>
                )}
                {allSubscriptions.length > activeSubscriptions.length && (
                  <Tag color='white' size='small' shape='circle'>
                    {allSubscriptions.length - activeSubscriptions.length}{' '}
                    {t('个已过期')}
                  </Tag>
                )}
              </div>
              <div className='flex items-center gap-2'>
                {hasActiveSubscription && (
                  <Button
                    size='small'
                    theme='light'
                    type='primary'
                    icon={<ListOrdered size={13} />}
                    onClick={openConsumptionOrder}
                  >
                    {t('消耗顺序')}
                  </Button>
                )}
                <Select
                  value={displayBillingPreference}
                  onChange={onChangeBillingPreference}
                  size='small'
                  optionList={[
                    {
                      value: 'subscription_first',
                      label: disableSubscriptionPreference
                        ? `${t('优先订阅')} (${t('无生效')})`
                        : t('优先订阅'),
                      disabled: disableSubscriptionPreference,
                    },
                    { value: 'wallet_first', label: t('优先钱包') },
                    {
                      value: 'subscription_only',
                      label: disableSubscriptionPreference
                        ? `${t('仅用订阅')} (${t('无生效')})`
                        : t('仅用订阅'),
                      disabled: disableSubscriptionPreference,
                    },
                    { value: 'wallet_only', label: t('仅用钱包') },
                  ]}
                />
                <Button
                  size='small'
                  theme='light'
                  type='tertiary'
                  icon={
                    <RefreshCw
                      size={12}
                      className={refreshing ? 'animate-spin' : ''}
                    />
                  }
                  onClick={handleRefresh}
                  loading={refreshing}
                />
              </div>
            </div>
            {disableSubscriptionPreference && isSubscriptionPreference && (
              <Text type='tertiary' size='small'>
                {t('已保存偏好为')}
                {subscriptionPreferenceLabel}
                {t('，当前无生效订阅，将自动使用钱包')}
              </Text>
            )}

            {hasAnySubscription ? (
              <>
                <Divider margin={8} />
                <div className='max-h-64 overflow-y-auto pr-1 semi-table-body'>
                  {allSubscriptions.map((sub, subIndex) => {
                    const isLast = subIndex === allSubscriptions.length - 1;
                    const subscription = sub.subscription;
                    const totalAmount = Number(subscription?.amount_total || 0);
                    const usedAmount = Number(subscription?.amount_used || 0);
                    const remainAmount =
                      totalAmount > 0
                        ? Math.max(0, totalAmount - usedAmount)
                        : 0;
                    const planTitle =
                      planTitleMap.get(subscription?.plan_id) || '';
                    const remainDays = getRemainingDays(sub);
                    const usagePercent = getUsagePercent(sub);
                    const now = Date.now() / 1000;
                    const isExpired = (subscription?.end_time || 0) < now;
                    const isCancelled = subscription?.status === 'cancelled';
                    const isActive =
                      subscription?.status === 'active' && !isExpired;

                    return (
                      <div key={subscription?.id || subIndex}>
                        {/* 订阅概要 */}
                        <div className='flex items-center justify-between text-xs mb-2'>
                          <div className='flex items-center gap-2'>
                            <span className='font-medium'>
                              {planTitle
                                ? `${planTitle} · ${t('订阅')} #${subscription?.id}`
                                : `${t('订阅')} #${subscription?.id}`}
                            </span>
                            {isActive ? (
                              <Tag
                                color='white'
                                size='small'
                                shape='circle'
                                prefixIcon={<Badge dot type='success' />}
                              >
                                {t('生效')}
                              </Tag>
                            ) : isCancelled ? (
                              <Tag color='white' size='small' shape='circle'>
                                {t('已作废')}
                              </Tag>
                            ) : (
                              <Tag color='white' size='small' shape='circle'>
                                {t('已过期')}
                              </Tag>
                            )}
                            {subscription?.hidden && (
                              <Tag color='grey' size='small' shape='circle'>
                                已隐藏
                              </Tag>
                            )}
                          </div>
                          <div className='flex items-center gap-2'>
                            {isActive && (
                              <span className='text-gray-500'>
                                {t('剩余')} {remainDays} {t('天')}
                              </span>
                            )}
                            {subscription?.hidden ? (
                              <Button
                                size='small'
                                type='primary'
                                theme='borderless'
                                loading={restoringId === subscription?.id}
                                onClick={() =>
                                  restoreSubscription(subscription.id)
                                }
                              >
                                恢复展示
                              </Button>
                            ) : (
                              <Button
                                size='small'
                                type='tertiary'
                                onClick={() =>
                                  setManageSubscription(subscription)
                                }
                              >
                                管理
                              </Button>
                            )}
                          </div>
                        </div>
                        <div className='text-xs text-gray-500 mb-2'>
                          {isActive
                            ? t('至')
                            : isCancelled
                              ? t('作废于')
                              : t('过期于')}{' '}
                          {new Date(
                            (subscription?.end_time || 0) * 1000,
                          ).toLocaleString()}
                        </div>
                        {isActive && subscription?.next_reset_time > 0 && (
                          <div className='text-xs text-gray-500 mb-2'>
                            {t('下一次重置')}:{' '}
                            {new Date(
                              subscription.next_reset_time * 1000,
                            ).toLocaleString()}
                          </div>
                        )}
                        <div className='text-xs text-gray-500 mb-2'>
                          {t('总额度')}:{' '}
                          {totalAmount > 0 ? (
                            <Tooltip
                              content={`${t('原生额度')}：${usedAmount}/${totalAmount} · ${t('剩余')} ${remainAmount}`}
                            >
                              <span>
                                {renderQuota(usedAmount)}/
                                {renderQuota(totalAmount)} · {t('剩余')}{' '}
                                {renderQuota(remainAmount)}
                              </span>
                            </Tooltip>
                          ) : (
                            t('不限')
                          )}
                          {totalAmount > 0 && (
                            <span className='ml-2'>
                              {t('已用')} {usagePercent}%
                            </span>
                          )}
                        </div>
                        {!isLast && <Divider margin={12} />}
                      </div>
                    );
                  })}
                </div>
              </>
            ) : (
              <div className='text-xs text-gray-500'>
                {t('购买套餐后即可享受模型权益')}
              </div>
            )}
          </Card>

          {/* 可购买套餐 - 标准定价卡片 */}
          {plans.length > 0 ? (
            <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 2xl:gap-5 w-full px-1'>
              {plans.map((p, index) => {
                const plan = p?.plan;
                const totalAmount = Number(plan?.total_amount || 0);
                const { symbol, rate } = getCurrencyConfig();
                const price = Number(plan?.price_amount || 0);
                const convertedPrice = price * rate;
                const displayPrice = convertedPrice.toFixed(
                  Number.isInteger(convertedPrice) ? 0 : 2,
                );
                const isPopular = index === 0 && plans.length > 1;
                const limit = Number(plan?.max_purchase_per_user || 0);
                const limitLabel = limit > 0 ? `${t('限购')} ${limit}` : null;
                const totalLabel =
                  totalAmount > 0
                    ? `${t('总额度')}: ${renderQuota(totalAmount)}`
                    : `${t('总额度')}: ${t('不限')}`;
                const upgradeLabel = plan?.upgrade_group
                  ? `${t('升级分组')}: ${plan.upgrade_group}`
                  : null;
                const resetLabel =
                  formatSubscriptionResetPeriod(plan, t) === t('不重置')
                    ? null
                    : `${t('额度重置')}: ${formatSubscriptionResetPeriod(plan, t)}`;
                const luckyCardGrantCount = Number(
                  plan?.lucky_card_grant_count || 0,
                );
                const planBenefits = [
                  {
                    label: `${t('有效期')}: ${formatSubscriptionDuration(plan, t)}`,
                  },
                  resetLabel ? { label: resetLabel } : null,
                  luckyCardGrantCount > 0
                    ? {
                        label: `${t('立即获得幸运卡')}: ${luckyCardGrantCount} ${t('张')}`,
                      }
                    : null,
                  plan?.lucky_card_on_reset
                    ? { label: `${t('周期重置')}: ${t('每次获得 1 张')}` }
                    : null,
                  totalAmount > 0
                    ? {
                        label: totalLabel,
                        tooltip: `${t('原生额度')}：${totalAmount}`,
                      }
                    : { label: totalLabel },
                  limitLabel ? { label: limitLabel } : null,
                  upgradeLabel ? { label: upgradeLabel } : null,
                ].filter(Boolean);

                return (
                  <Card
                    key={plan?.id}
                    className={`!rounded-xl transition-all hover:shadow-lg w-full h-full ${
                      isPopular ? 'ring-2 ring-purple-500' : ''
                    }`}
                    bodyStyle={{ padding: 0 }}
                  >
                    <div className='p-4 h-full flex flex-col'>
                      {/* 推荐标签 */}
                      {isPopular && (
                        <div className='mb-2'>
                          <Tag color='purple' shape='circle' size='small'>
                            <Sparkles size={10} className='mr-1' />
                            {t('推荐')}
                          </Tag>
                        </div>
                      )}
                      {/* 套餐名称 */}
                      <div className='mb-3'>
                        <Typography.Title
                          heading={5}
                          ellipsis={{ rows: 1, showTooltip: true }}
                          style={{ margin: 0 }}
                        >
                          {plan?.title || t('订阅套餐')}
                        </Typography.Title>
                        {plan?.subtitle && (
                          <Text
                            type='tertiary'
                            size='small'
                            ellipsis={{ rows: 3, showTooltip: true }}
                            style={{ display: 'block' }}
                          >
                            {plan.subtitle}
                          </Text>
                        )}
                      </div>

                      {/* 价格区域 */}
                      <div className='py-2'>
                        <div className='flex items-baseline justify-start'>
                          <span className='text-xl font-bold text-purple-600'>
                            {symbol}
                          </span>
                          <span className='text-3xl font-bold text-purple-600'>
                            {displayPrice}
                          </span>
                        </div>
                      </div>

                      {/* 套餐权益描述 */}
                      <div className='flex flex-col items-start gap-1 pb-2'>
                        {planBenefits.map((item) => {
                          const content = (
                            <div className='flex items-center gap-2 text-xs text-gray-500'>
                              <Badge dot type='tertiary' />
                              <span>{item.label}</span>
                            </div>
                          );
                          if (!item.tooltip) {
                            return (
                              <div
                                key={item.label}
                                className='w-full flex justify-start'
                              >
                                {content}
                              </div>
                            );
                          }
                          return (
                            <Tooltip key={item.label} content={item.tooltip}>
                              <div className='w-full flex justify-start'>
                                {content}
                              </div>
                            </Tooltip>
                          );
                        })}
                      </div>

                      <div className='mt-auto'>
                        <Divider margin={12} />

                        {/* 购买按钮 */}
                        {(() => {
                          const count = getPlanPurchaseCount(p?.plan?.id);
                          const reached = limit > 0 && count >= limit;
                          const tip = reached
                            ? t('已达到购买上限') + ` (${count}/${limit})`
                            : '';
                          const buttonEl = (
                            <Button
                              theme='outline'
                              type='primary'
                              block
                              disabled={reached}
                              onClick={() => {
                                if (!reached) openBuy(p);
                              }}
                            >
                              {reached ? t('已达上限') : t('立即订阅')}
                            </Button>
                          );
                          return reached ? (
                            <Tooltip content={tip} position='top'>
                              {buttonEl}
                            </Tooltip>
                          ) : (
                            buttonEl
                          );
                        })()}
                      </div>
                    </div>
                  </Card>
                );
              })}
            </div>
          ) : (
            <div className='text-center text-gray-400 text-sm py-4'>
              {t('暂无可购买套餐')}
            </div>
          )}
        </Space>
      )}
    </>
  );

  return (
    <>
      {withCard ? (
        <Card className='!rounded-2xl shadow-sm border-0'>{cardContent}</Card>
      ) : (
        <div className='space-y-3'>{cardContent}</div>
      )}

      {/* 购买确认弹窗 */}
      <SubscriptionPurchaseModal
        t={t}
        visible={open}
        onCancel={closeBuy}
        selectedPlan={selectedPlan}
        paying={paying}
        selectedEpayMethod={selectedEpayMethod}
        setSelectedEpayMethod={setSelectedEpayMethod}
        epayMethods={epayMethods}
        enableOnlineTopUp={enableOnlineTopUp}
        enableStripeTopUp={enableStripeTopUp}
        enableCreemTopUp={enableCreemTopUp}
        purchaseLimitInfo={
          selectedPlan?.plan?.id
            ? {
                limit: Number(selectedPlan?.plan?.max_purchase_per_user || 0),
                count: getPlanPurchaseCount(selectedPlan?.plan?.id),
              }
            : null
        }
        onPayStripe={payStripe}
        onPayCreem={payCreem}
        onPayEpay={payEpay}
      />

      <Modal
        visible={orderOpen}
        title={
          <div className='flex items-center gap-2'>
            <ListOrdered size={18} />
            <span>{t('套餐消耗顺序')}</span>
          </div>
        }
        onCancel={() => setOrderOpen(false)}
        onOk={saveConsumptionOrder}
        confirmLoading={orderSaving}
        okButtonProps={{
          disabled: orderLoading || orderItems.length === 0,
        }}
        okText={t('保存顺序')}
        cancelText={t('取消')}
        width={560}
      >
        <Text type='tertiary' size='small'>
          {t(
            '未绑定具体实例的 API Key 按此顺序消耗；已绑定实例的 Key 不受影响。',
          )}
        </Text>
        <Select
          className='mt-4 w-full'
          value={orderGroup}
          optionList={subscriptionGroups.map((group) => ({
            value: group,
            label: group === 'default' ? t('默认分组') : group,
          }))}
          onChange={(group) => {
            setOrderGroup(group);
            loadConsumptionOrder(group);
          }}
        />
        <Spin spinning={orderLoading}>
          <div className='mt-3 max-h-80 overflow-y-auto space-y-2 pr-1'>
            {orderItems.map((item, index) => (
              <div
                key={item.id}
                className='flex items-center gap-3 rounded-xl border border-gray-200 p-3'
              >
                <span className='grid h-7 w-7 shrink-0 place-items-center rounded-full bg-orange-50 font-mono text-xs font-semibold text-orange-600'>
                  {index + 1}
                </span>
                <div className='min-w-0 flex-1'>
                  <div className='truncate text-sm font-medium'>
                    {item.title}
                  </div>
                  <div className='mt-0.5 text-xs text-gray-500'>
                    #{item.id} · {t('到期')}{' '}
                    {new Date(item.endTime * 1000).toLocaleDateString()}
                  </div>
                </div>
                <Button
                  theme='borderless'
                  icon={<ArrowUp size={15} />}
                  disabled={index === 0}
                  onClick={() => moveConsumptionItem(index, -1)}
                  aria-label={t('上移')}
                />
                <Button
                  theme='borderless'
                  icon={<ArrowDown size={15} />}
                  disabled={index === orderItems.length - 1}
                  onClick={() => moveConsumptionItem(index, 1)}
                  aria-label={t('下移')}
                />
              </div>
            ))}
            {!orderLoading && orderItems.length === 0 && (
              <div className='rounded-xl border border-dashed border-gray-200 py-10 text-center text-sm text-gray-400'>
                {t('当前分组没有可用套餐')}
              </div>
            )}
          </div>
        </Spin>
        <div className='mt-3 flex items-start gap-2 rounded-lg bg-gray-50 p-3 text-xs leading-relaxed text-gray-500'>
          <KeyRound size={14} className='mt-0.5 shrink-0' />
          <span>
            {t('调整顺序不会修改 API Key 的实例绑定，也不会移动历史消费记录。')}
          </span>
        </div>
      </Modal>

      <Modal
        visible={Boolean(manageSubscription)}
        title={
          <div className='flex items-center gap-2'>
            <KeyRound size={18} />
            <span>{t('管理套餐')}</span>
          </div>
        }
        onCancel={() => setManageSubscription(null)}
        footer={
          <div className='flex items-center justify-end gap-2'>
            <Button
              theme='borderless'
              onClick={() => setManageSubscription(null)}
            >
              {t('取消')}
            </Button>
            <Button
              theme='solid'
              type='danger'
              loading={manageHiding}
              onClick={hideFromManage}
            >
              隐藏此套餐
            </Button>
          </div>
        }
        width={480}
      >
        {manageSubscription && (
          <div className='space-y-3'>
            <div className='flex items-center justify-between text-sm'>
              <span className='text-gray-500'>套餐</span>
              <span className='font-medium'>
                {manageSubscription?.plan_title ||
                  `${t('订阅')} #${manageSubscription?.id}`}
              </span>
            </div>
            <div className='flex items-center justify-between text-sm'>
              <span className='text-gray-500'>订阅编号</span>
              <span className='font-medium'>#{manageSubscription?.id}</span>
            </div>
            <div className='flex items-center justify-between text-sm'>
              <span className='text-gray-500'>有效期至</span>
              <span className='font-medium'>
                {new Date(
                  (manageSubscription?.end_time || 0) * 1000,
                ).toLocaleString()}
              </span>
            </div>
            <div className='rounded-lg border border-orange-200 bg-orange-50 p-3 text-xs text-orange-700'>
              只从你的剩余用量页面隐藏，不会影响额度、绑定或实际使用。
            </div>
          </div>
        )}
      </Modal>
    </>
  );
};

export default SubscriptionPlansCard;
