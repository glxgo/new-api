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

import React, { useEffect, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Modal,
  Progress,
  Radio,
  RadioGroup,
  Tag,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';

const VirtualMembership = () => {
  const [renderedAt] = useState(() => Date.now() / 1000);
  const [page, setPage] = useState({
    announcement: '',
    plans: [],
    memberships: [],
    epay_enabled: false,
    epay_methods: [],
  });
  const [loading, setLoading] = useState(true);
  const [selectedGroup, setSelectedGroup] = useState({});
  const [purchase, setPurchase] = useState(null);
  const [paymentMethod, setPaymentMethod] = useState('balance');
  const [paying, setPaying] = useState(false);
  const [manageMembership, setManageMembership] = useState(null);
  const [manageHiding, setManageHiding] = useState(false);
  const [restoringId, setRestoringId] = useState(null);
  const [resettingId, setResettingId] = useState(null);

  const load = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/virtual-membership/page');
      if (res.data?.success) {
        const nextPage = res.data.data || page;
        setPage(nextPage);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const openPurchase = (plan, groupSize) => {
    setPurchase({ plan, groupSize, renewFromMembershipId: null });
    setPaymentMethod(
      page.epay_methods?.[0]?.type
        ? `epay:${page.epay_methods[0].type}`
        : 'balance',
    );
  };

  const hideMembership = async (membershipId) => {
    try {
      const res = await API.patch(
        `/api/virtual-membership/${membershipId}/visibility`,
        { hidden: true },
      );
      if (res.data?.success) {
        showSuccess('已隐藏该会员，可联系管理员恢复展示');
        await load();
      } else {
        showError(res.data?.message || '隐藏失败');
      }
    } catch (error) {
      showError(error);
    }
  };

  const hideFromManage = async () => {
    if (!manageMembership?.id) return;
    setManageHiding(true);
    try {
      await hideMembership(manageMembership.id);
      setManageMembership(null);
    } finally {
      setManageHiding(false);
    }
  };

  const restoreMembership = async (membershipId) => {
    setRestoringId(membershipId);
    try {
      const res = await API.patch(
        `/api/virtual-membership/${membershipId}/visibility`,
        { hidden: false },
      );
      if (res.data?.success) {
        showSuccess('已恢复展示');
        await load();
      } else {
        showError(res.data?.message || '恢复展示失败');
      }
    } catch (error) {
      showError(error);
    } finally {
      setRestoringId(null);
    }
  };

  const submitEpayForm = (payload) => {
    if (!payload?.url) return false;
    const form = document.createElement('form');
    form.action = payload.url;
    form.method = 'POST';
    const isSafari =
      typeof navigator !== 'undefined' &&
      /^((?!chrome|android).)*safari/i.test(navigator.userAgent);
    if (!isSafari) form.target = '_blank';
    Object.entries(payload.data || {}).forEach(([key, value]) => {
      const input = document.createElement('input');
      input.type = 'hidden';
      input.name = key;
      input.value = String(value);
      form.appendChild(input);
    });
    document.body.appendChild(form);
    form.submit();
    form.remove();
    return true;
  };

  const activeReset = async (membership) => {
    if (resettingId !== null) return;
    setResettingId(membership.id);
    try {
      if (Number(membership.active_reset_credits || 0) > 0) {
        if (
          !window.confirm(
            '确认立即重置该虚拟会员的周限额和 5 小时限额吗？将消耗 1 次主动重置次数。',
          )
        )
          return;
        const res = await API.post(
          `/api/virtual-membership/${membership.id}/reset`,
        );
        if (res.data?.success) {
          showSuccess('已主动重置额度');
          await load();
        } else {
          showError(new Error(res.data?.message || '主动重置失败'));
        }
        return;
      }
      const method = page.epay_methods?.[0]?.type;
      const price = Number(membership.active_reset_price_amount || 0);
      if (!method) {
        showError(new Error('当前未配置易支付方式'));
        return;
      }
      if (price < 0.01) {
        showError(new Error('主动重置价格暂不可用，请联系管理员'));
        return;
      }
      if (
        !window.confirm(
          `主动重置次数不足，是否购买 1 次？价格 ¥${price.toFixed(2)}`,
        )
      )
        return;
      const res = await API.post(
        `/api/virtual-membership/${membership.id}/reset/epay`,
        { payment_method: method },
      );
      if (res.data?.message !== 'success' || !submitEpayForm(res.data)) {
        showError(new Error(res.data?.message || '支付请求失败'));
        return;
      }
      showSuccess('支付页面已打开，支付成功后将获得 1 次主动重置');
    } catch (error) {
      showError(error);
    } finally {
      setResettingId(null);
    }
  };

  const openRenewal = (membership) => {
    const plan = page.plans?.find((item) => item.id === membership.plan_id);
    if (!plan) {
      showError(new Error('当前会员方案已下架，请联系管理员续费'));
      return;
    }
    setPurchase({
      plan,
      groupSize: membership.group_size,
      renewFromMembershipId: membership.id,
    });
    setPaymentMethod(
      page.epay_methods?.[0]?.type
        ? `epay:${page.epay_methods[0].type}`
        : 'balance',
    );
  };

  const confirmPurchase = async () => {
    if (!purchase) return;
    setPaying(true);
    try {
      if (paymentMethod === 'balance') {
        const res = await API.post('/api/virtual-membership/balance/pay', {
          plan_id: purchase.plan.id,
          group_size: purchase.groupSize,
          renew_from_membership_id: purchase.renewFromMembershipId || undefined,
        });
        if (res.data?.success) {
          showSuccess(
            purchase.renewFromMembershipId ? '续费已创建' : '虚拟会员已开通',
          );
          setPurchase(null);
          await load();
        }
        return;
      }
      const selectedEpayMethod = paymentMethod.replace(/^epay:/, '');
      const res = await API.post('/api/virtual-membership/epay/pay', {
        plan_id: purchase.plan.id,
        group_size: purchase.groupSize,
        payment_method: selectedEpayMethod,
        renew_from_membership_id: purchase.renewFromMembershipId || undefined,
      });
      if (res.data?.message === 'success' && res.data?.url) {
        const form = document.createElement('form');
        form.action = res.data.url;
        form.method = 'POST';
        const isSafari =
          typeof navigator !== 'undefined' &&
          /^((?!chrome|android).)*safari/i.test(navigator.userAgent);
        if (!isSafari) form.target = '_blank';
        Object.entries(res.data.data || {}).forEach(([key, value]) => {
          const input = document.createElement('input');
          input.type = 'hidden';
          input.name = key;
          input.value = String(value);
          form.appendChild(input);
        });
        document.body.appendChild(form);
        form.submit();
        form.remove();
        showSuccess('支付页面已打开');
        setPurchase(null);
      } else {
        showError(new Error(res.data?.message || '支付请求失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setPaying(false);
    }
  };

  const variantFor = (plan) => {
    const groupSize = selectedGroup[plan.id] || 1;
    return {
      groupSize,
      variant:
        plan.variants?.find((item) => item.group_size === groupSize) ||
        plan.variants?.[0],
    };
  };

  return (
    <div className='mt-[60px] px-2'>
      <div className='mx-auto max-w-7xl space-y-4'>
        <div>
          <h1 className='text-2xl font-semibold'>虚拟会员</h1>
          <p className='text-gray-500'>
            购买对应额度，不提供真实 GPT 会员账号。
          </p>
        </div>
        {page.announcement?.trim() && (
          <Banner
            type='info'
            description={
              <div style={{ whiteSpace: 'pre-wrap' }}>{page.announcement}</div>
            }
            closeIcon={null}
          />
        )}
        <Modal
          title={purchase?.renewFromMembershipId ? '续费虚拟会员' : '立即购买'}
          visible={!!purchase}
          onCancel={() => setPurchase(null)}
          onOk={confirmPurchase}
          okText='确认支付'
          cancelText='取消'
          confirmLoading={paying}
          maskClosable={false}
          width={460}
        >
          {purchase &&
            (() => {
              const { plan, groupSize } = purchase;
              const variant =
                plan.variants?.find((item) => item.group_size === groupSize) ||
                plan.variants?.[0];
              return (
                <div className='space-y-4'>
                  <div className='rounded border bg-gray-50 p-3'>
                    <div className='flex justify-between'>
                      <span>方案</span>
                      <strong>{plan.title}</strong>
                    </div>
                    <div className='mt-2 flex justify-between'>
                      <span>购买档位</span>
                      <strong>{variant?.label}</strong>
                    </div>
                    <div className='mt-2 flex justify-between'>
                      <span>应付金额</span>
                      <strong className='text-green-600'>
                        ¥{Number(variant?.price_amount || 0).toFixed(2)}
                      </strong>
                    </div>
                    <div className='mt-2 flex justify-between'>
                      <span>获得周额度</span>
                      <strong>{variant?.weekly_quota || 0}</strong>
                    </div>
                    {plan.five_hour_enabled && (
                      <div className='mt-2 flex justify-between'>
                        <span>获得 5 小时额度</span>
                        <strong>{variant?.five_hour_quota || 0}</strong>
                      </div>
                    )}
                  </div>
                  <div>
                    <div className='mb-2 text-sm font-medium'>付款方式</div>
                    <RadioGroup
                      type='button'
                      value={paymentMethod}
                      onChange={(event) => setPaymentMethod(event.target.value)}
                      style={{ width: '100%' }}
                    >
                      <Radio value='balance'>钱包余额</Radio>
                      {page.epay_methods?.map((method) => (
                        <Radio key={method.type} value={`epay:${method.type}`}>
                          {method.name || method.type}
                        </Radio>
                      ))}
                    </RadioGroup>
                  </div>
                </div>
              );
            })()}
        </Modal>
        {page.memberships?.length > 0 && (
          <div>
            <h2 className='mb-3 text-lg font-semibold'>我的虚拟会员</h2>
            <div className='grid gap-4 md:grid-cols-2'>
              {page.memberships.map((item) => (
                <Card
                  key={item.id}
                  title={item.plan_title}
                  headerExtraContent={
                    <Tag
                      color={item.start_time > renderedAt ? 'orange' : 'green'}
                    >
                      {item.start_time > renderedAt ? '待生效' : '生效中'}
                    </Tag>
                  }
                >
                  <div className='space-y-3'>
                    <div>
                      <div className='mb-1 flex justify-between text-sm'>
                        <span>周限额</span>
                        <span>
                          {item.weekly_remaining} / {item.weekly_quota}
                        </span>
                      </div>
                      <Progress percent={item.weekly_percent} />
                    </div>
                    {item.five_hour_enabled && (
                      <div>
                        <div className='mb-1 flex justify-between text-sm'>
                          <span>5 小时限额</span>
                          <span>
                            {item.five_hour_remaining} / {item.five_hour_quota}
                          </span>
                        </div>
                        <Progress percent={item.five_hour_percent} />
                      </div>
                    )}
                    <p className='text-xs text-gray-500'>
                      {item.start_time > renderedAt ? '生效时间' : '有效期至'}{' '}
                      {new Date(
                        (item.start_time > renderedAt
                          ? item.start_time
                          : item.end_time) * 1000,
                      ).toLocaleString()}
                    </p>
                    <div className='grid grid-cols-2 gap-2'>
                      <Button
                        theme='outline'
                        className='w-full'
                        onClick={() => setManageMembership(item)}
                      >
                        管理
                      </Button>
                      <Button
                        theme='borderless'
                        className='w-full'
                        disabled={
                          page.memberships.some(
                            (candidate) =>
                              candidate.renewed_from_id === item.id,
                          ) ||
                          !page.plans.some((plan) => plan.id === item.plan_id)
                        }
                        onClick={() => openRenewal(item)}
                      >
                        续费
                      </Button>
                    </div>
                    <Button
                      theme='outline'
                      className='mt-2 w-full'
                      loading={resettingId === item.id}
                      disabled={
                        item.start_time > renderedAt || item.status !== 'active'
                      }
                      onClick={() => activeReset(item)}
                    >
                      {Number(item.active_reset_credits || 0) > 0
                        ? `主动重置（剩余 ${item.active_reset_credits} 次）`
                        : `购买主动重置次数（¥${Number(item.active_reset_price_amount || 0).toFixed(2)}）`}
                    </Button>
                  </div>
                </Card>
              ))}
            </div>
          </div>
        )}
        {page.hidden_memberships?.length > 0 && (
          <div>
            <h2 className='mb-3 text-lg font-semibold'>已隐藏的虚拟会员</h2>
            <div className='grid gap-4 md:grid-cols-2'>
              {page.hidden_memberships.map((item) => (
                <Card
                  key={item.id}
                  title={item.plan_title}
                  headerExtraContent={
                    <Tag color='grey' size='small'>
                      已隐藏
                    </Tag>
                  }
                >
                  <div className='space-y-3'>
                    <p className='text-xs text-gray-500'>
                      有效期至 {new Date(item.end_time * 1000).toLocaleString()}{' '}
                      · 会员 #{item.id}
                    </p>
                    <Button
                      theme='outline'
                      className='w-full'
                      loading={restoringId === item.id}
                      onClick={() => restoreMembership(item.id)}
                    >
                      恢复展示
                    </Button>
                  </div>
                </Card>
              ))}
            </div>
          </div>
        )}
        <div>
          <h2 className='mb-3 text-lg font-semibold'>选择方案</h2>
          {loading ? (
            <div className='py-10 text-center text-gray-500'>加载中…</div>
          ) : (
            <div className='grid gap-4 lg:grid-cols-3'>
              {page.plans.map((plan) => {
                const { groupSize } = variantFor(plan);
                return (
                  <Card
                    key={plan.id}
                    title={plan.title}
                    headerExtraContent={
                      plan.recommended ? <Tag color='green'>推荐</Tag> : null
                    }
                  >
                    <p className='mb-4 text-sm text-gray-500'>
                      {plan.subtitle || `有效期 ${plan.duration_days} 天`}
                    </p>
                    {plan.description && (
                      <div
                        className='mb-4 text-sm text-gray-500'
                        style={{ whiteSpace: 'pre-wrap' }}
                      >
                        {plan.description}
                      </div>
                    )}
                    <div className='overflow-hidden rounded border text-sm'>
                      <div className='grid grid-cols-2 gap-2 border-b bg-gray-50 px-3 py-2 font-medium text-gray-500'>
                        <span>购买档位 / 价格</span>
                        <span className='text-right'>周额度</span>
                      </div>
                      {(plan.variants || []).map((item) => (
                        <button
                          key={item.group_size}
                          type='button'
                          className={`grid w-full grid-cols-2 gap-2 border-b px-3 py-3 text-left last:border-b-0 ${
                            groupSize === item.group_size
                              ? 'bg-green-50'
                              : 'hover:bg-gray-50'
                          }`}
                          onClick={() =>
                            setSelectedGroup({
                              ...selectedGroup,
                              [plan.id]: item.group_size,
                            })
                          }
                        >
                          <span>
                            <span className='block font-medium'>
                              {item.label}
                            </span>
                            <span className='mt-1 block font-semibold text-green-600'>
                              ¥{Number(item.price_amount || 0).toFixed(2)}
                            </span>
                          </span>
                          <span className='text-right font-semibold'>
                            {item.weekly_quota || 0}
                            {plan.five_hour_enabled && (
                              <span className='mt-1 block text-xs font-normal text-gray-500'>
                                5h {item.five_hour_quota || 0}
                              </span>
                            )}
                          </span>
                        </button>
                      ))}
                    </div>
                    <Button
                      theme='solid'
                      type='primary'
                      className='mt-4 w-full'
                      onClick={() => openPurchase(plan, groupSize)}
                    >
                      立即购买
                    </Button>
                  </Card>
                );
              })}
            </div>
          )}
        </div>
      </div>
      <Modal
        title='管理虚拟会员'
        visible={Boolean(manageMembership)}
        onCancel={() => setManageMembership(null)}
        footer={
          <div className='flex items-center justify-end gap-2'>
            <Button
              theme='borderless'
              onClick={() => setManageMembership(null)}
            >
              取消
            </Button>
            <Button
              theme='solid'
              type='danger'
              loading={manageHiding}
              onClick={hideFromManage}
            >
              隐藏此会员
            </Button>
          </div>
        }
        width={460}
      >
        {manageMembership && (
          <div className='space-y-3'>
            <div className='flex items-center justify-between text-sm'>
              <span className='text-gray-500'>方案</span>
              <span className='font-medium'>{manageMembership.plan_title}</span>
            </div>
            <div className='flex items-center justify-between text-sm'>
              <span className='text-gray-500'>购买档位</span>
              <span className='font-medium'>
                {manageMembership.group_size === 1
                  ? '单独购买'
                  : `${manageMembership.group_size} 人团`}
              </span>
            </div>
            <div className='flex items-center justify-between text-sm'>
              <span className='text-gray-500'>会员编号</span>
              <span className='font-medium'>#{manageMembership.id}</span>
            </div>
            <div className='flex items-center justify-between text-sm'>
              <span className='text-gray-500'>有效期至</span>
              <span className='font-medium'>
                {new Date(manageMembership.end_time * 1000).toLocaleString()}
              </span>
            </div>
            <div className='rounded-lg border border-orange-200 bg-orange-50 p-3 text-xs text-orange-700'>
              只从你的剩余用量页面隐藏，不会影响额度、绑定或实际使用。
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default VirtualMembership;
