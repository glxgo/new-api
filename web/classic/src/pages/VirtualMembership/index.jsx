import React, { useEffect, useState } from 'react';
import { Banner, Button, Card, Progress, Tag } from '@douyinfe/semi-ui';
import { API } from '../../helpers';
import { showError } from '../../helpers';

const VirtualMembership = () => {
  const [page, setPage] = useState({
    announcement: '',
    plans: [],
    memberships: [],
    epay_enabled: false,
    epay_methods: [],
  });
  const [loading, setLoading] = useState(true);
  const [selectedGroup, setSelectedGroup] = useState({});
  const [selectedEpayMethod, setSelectedEpayMethod] = useState('');

  const load = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/virtual-membership/page');
      if (res.data?.success) {
        const nextPage = res.data.data || page;
        setPage(nextPage);
        setSelectedEpayMethod(nextPage.epay_methods?.[0]?.type || '');
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

  const purchase = async (plan, groupSize) => {
    if (
      !window.confirm(
        `确认购买 ${plan.title}（${groupSize === 1 ? '单独购买' : `${groupSize} 人团`}）？`,
      )
    )
      return;
    try {
      const res = await API.post('/api/virtual-membership/balance/pay', {
        plan_id: plan.id,
        group_size: groupSize,
      });
      if (res.data?.success) {
        await load();
      }
    } catch (error) {
      showError(error);
    }
  };

  const purchaseWithEpay = async (plan, groupSize) => {
    if (!selectedEpayMethod) {
      showError(new Error('暂无可用的支付宝支付方式'));
      return;
    }
    if (
      !window.confirm(
        `确认购买 ${plan.title}（${groupSize === 1 ? '单独购买' : `${groupSize} 人团`}）并使用支付宝支付？`,
      )
    )
      return;
    try {
      const res = await API.post('/api/virtual-membership/epay/pay', {
        plan_id: plan.id,
        group_size: groupSize,
        payment_method: selectedEpayMethod,
      });
      if (res.data?.message === 'success' && res.data?.url) {
        const form = document.createElement('form');
        form.action = res.data.url;
        form.method = 'POST';
        form.target = '_blank';
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
      } else {
        showError(new Error(res.data?.message || '支付请求失败'));
      }
    } catch (error) {
      showError(error);
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
        {page.announcement && (
          <Banner
            type='info'
            description={page.announcement}
            closeIcon={null}
          />
        )}
        {page.epay_enabled && page.epay_methods?.length > 0 && (
          <div className='flex flex-wrap items-center gap-3 rounded border border-green-200 bg-green-50 p-3'>
            <span className='text-sm font-medium'>支付宝支付方式</span>
            <select
              value={selectedEpayMethod}
              onChange={(event) => setSelectedEpayMethod(event.target.value)}
              className='rounded border bg-white px-3 py-2 text-sm'
            >
              {page.epay_methods.map((method) => (
                <option key={method.type} value={method.type}>
                  {method.name || method.type}
                </option>
              ))}
            </select>
            <span className='text-xs text-gray-500'>
              复用订阅套餐的 Epay 配置
            </span>
          </div>
        )}
        {page.memberships?.length > 0 && (
          <div>
            <h2 className='mb-3 text-lg font-semibold'>我的虚拟会员</h2>
            <div className='grid gap-4 md:grid-cols-2'>
              {page.memberships.map((item) => (
                <Card
                  key={item.id}
                  title={item.plan_title}
                  headerExtraContent={<Tag color='green'>生效中</Tag>}
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
                      有效期至 {new Date(item.end_time * 1000).toLocaleString()}
                    </p>
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
                const { groupSize, variant } = variantFor(plan);
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
                    <div className='grid grid-cols-2 gap-2 text-sm'>
                      <div className='rounded bg-green-50 p-3'>
                        <span className='text-gray-500'>周额度</span>
                        <strong className='mt-1 block'>
                          {variant?.weekly_quota || 0}
                        </strong>
                      </div>
                      <div className='rounded bg-blue-50 p-3'>
                        <span className='text-gray-500'>5 小时额度</span>
                        <strong className='mt-1 block'>
                          {plan.five_hour_enabled
                            ? variant?.five_hour_quota || 0
                            : '关闭'}
                        </strong>
                      </div>
                    </div>
                    <div className='mt-4 grid grid-cols-2 gap-2'>
                      {(plan.variants || []).map((item) => (
                        <Button
                          key={item.group_size}
                          type={
                            groupSize === item.group_size
                              ? 'primary'
                              : 'tertiary'
                          }
                          onClick={() =>
                            setSelectedGroup({
                              ...selectedGroup,
                              [plan.id]: item.group_size,
                            })
                          }
                        >
                          {item.label} ¥
                          {Number(item.price_amount || 0).toFixed(2)}
                        </Button>
                      ))}
                    </div>
                    <div className='mt-4 grid gap-2 sm:grid-cols-2'>
                      <Button
                        theme='outline'
                        type='primary'
                        onClick={() => purchase(plan, groupSize)}
                      >
                        钱包余额购买
                      </Button>
                      {page.epay_enabled && page.epay_methods?.length > 0 && (
                        <Button
                          theme='solid'
                          type='primary'
                          onClick={() => purchaseWithEpay(plan, groupSize)}
                        >
                          支付宝支付
                        </Button>
                      )}
                    </div>
                  </Card>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default VirtualMembership;
