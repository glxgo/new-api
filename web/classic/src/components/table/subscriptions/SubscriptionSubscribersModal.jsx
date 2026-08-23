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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Empty,
  Input,
  Progress,
  Select,
  SideSheet,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconRefresh, IconSearch, IconUserGroup } from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { API, renderQuota, showError } from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import CardTable from '../../common/ui/CardTable';

const { Text } = Typography;

const PAGE_SIZE = 10;

const toFiniteNumber = (value) => {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
};

const formatTimestamp = (timestamp) => {
  const value = toFiniteNumber(timestamp);
  return value > 0 ? new Date(value * 1000).toLocaleString() : '-';
};

const isResetDue = (item, now) => {
  return (
    item?.reset_due === true ||
    item?.reset_due === 'true' ||
    (toFiniteNumber(item?.next_reset_time) > 0 &&
      toFiniteNumber(item?.next_reset_time) <= now)
  );
};

// The API already applies this predicate. Keep the same guard in Classic so
// stale responses or a future backend change cannot reintroduce expired or
// exhausted instances into the management view.
const isUsable = (item, now) => {
  if (!item || item.status !== 'active') return false;

  const startTime = toFiniteNumber(item.start_time);
  const endTime = toFiniteNumber(item.end_time);
  if ((startTime > 0 && startTime > now) || endTime <= now) return false;

  const cycleTotal = toFiniteNumber(item.amount_total);
  const cycleUsed = Math.max(0, toFiniteNumber(item.amount_used));
  const capTotal = toFiniteNumber(item.amount_cap);
  const capUsed = Math.max(0, toFiniteNumber(item.amount_cap_used));
  const cycleAvailable =
    cycleTotal <= 0 || cycleUsed < cycleTotal || isResetDue(item, now);
  const capAvailable = capTotal <= 0 || capUsed < capTotal;

  return cycleAvailable && capAvailable;
};

const formatResetPeriod = (item, t) => {
  const period = item?.quota_reset_period || 'never';
  if (period === 'daily') return t('每天');
  if (period === 'weekly') return t('每周');
  if (period === 'monthly') return t('每月');
  if (period === 'custom') {
    const seconds = toFiniteNumber(item?.quota_reset_custom_seconds);
    if (seconds >= 86400) return `${Math.floor(seconds / 86400)} ${t('天')}`;
    if (seconds >= 3600) return `${Math.floor(seconds / 3600)} ${t('小时')}`;
    if (seconds >= 60) return `${Math.floor(seconds / 60)} ${t('分钟')}`;
    return `${seconds} ${t('秒')}`;
  }
  return t('不重置');
};

const renderStatus = (item, t) => {
  if (item?.status === 'active') {
    return (
      <Tag color='green' shape='circle' size='small'>
        {t('生效')}
      </Tag>
    );
  }
  if (item?.status === 'cancelled') {
    return (
      <Tag color='grey' shape='circle' size='small'>
        {t('已作废')}
      </Tag>
    );
  }
  return (
    <Tag color='grey' shape='circle' size='small'>
      {t('已过期')}
    </Tag>
  );
};

const QuotaCell = ({ item, t }) => {
  const cycleTotal = toFiniteNumber(item?.amount_total);
  const cycleUsed = Math.max(0, toFiniteNumber(item?.amount_used));
  const cycleRemaining =
    cycleTotal > 0 ? Math.max(0, cycleTotal - cycleUsed) : 0;
  const capTotal = toFiniteNumber(item?.amount_cap);
  const capUsed = Math.max(0, toFiniteNumber(item?.amount_cap_used));
  const percent =
    cycleTotal > 0
      ? Math.min(100, Math.max(0, (cycleRemaining / cycleTotal) * 100))
      : 0;
  const nextReset = toFiniteNumber(item?.next_reset_time);

  return (
    <div className='min-w-[190px]'>
      <div className='flex items-baseline justify-between gap-2'>
        <Text strong>
          {cycleTotal > 0 ? renderQuota(cycleRemaining) : t('不限')}
        </Text>
        {cycleTotal > 0 && (
          <Text type='tertiary'>/ {renderQuota(cycleTotal)}</Text>
        )}
      </div>
      {cycleTotal > 0 && (
        <Progress
          percent={percent}
          showInfo={false}
          style={{ width: '100%', marginTop: 4, marginBottom: 4 }}
        />
      )}
      <div className='text-xs text-gray-500'>
        {t('重置周期')}: {formatResetPeriod(item, t)}
      </div>
      <div className='text-xs text-gray-500'>
        {t('下一次重置')}:{' '}
        {nextReset > 0 ? formatTimestamp(nextReset) : t('不重置')}
      </div>
      {capTotal > 0 && (
        <div className='text-xs text-gray-500'>
          {t('总额度')}: {t('已用额度')} {renderQuota(capUsed)} /{' '}
          {renderQuota(capTotal)}
        </div>
      )}
    </div>
  );
};

const SubscriptionSubscribersModal = ({ visible, onCancel, t }) => {
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [items, setItems] = useState([]);
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [currentPage, setCurrentPage] = useState(1);
  const [now, setNow] = useState(() => Date.now() / 1000);

  const loadSubscribers = useCallback(
    async ({ resetLoading = true } = {}) => {
      if (resetLoading) setLoading(true);
      setRefreshing(true);
      try {
        const res = await API.get('/api/subscription/admin/subscribers');
        if (res.data?.success) {
          setItems(Array.isArray(res.data.data) ? res.data.data : []);
          setCurrentPage(1);
        } else {
          showError(res.data?.message || t('加载失败'));
        }
      } catch (error) {
        showError(t('请求失败'));
      } finally {
        if (resetLoading) setLoading(false);
        setRefreshing(false);
      }
    },
    [t],
  );

  useEffect(() => {
    if (!visible) return undefined;
    setKeyword('');
    setStatusFilter('all');
    setCurrentPage(1);
    setNow(Date.now() / 1000);
    loadSubscribers();

    const timer = window.setInterval(() => {
      setNow(Date.now() / 1000);
    }, 60 * 1000);
    return () => window.clearInterval(timer);
  }, [visible, loadSubscribers]);

  useEffect(() => {
    setCurrentPage(1);
  }, [keyword, statusFilter]);

  const usableItems = useMemo(
    () => (items || []).filter((item) => isUsable(item, now)),
    [items, now],
  );

  const filteredItems = useMemo(() => {
    const query = keyword.trim().toLocaleLowerCase();
    return usableItems.filter((item) => {
      if (statusFilter === 'active' && item.status !== 'active') return false;
      if (!query) return true;
      return [
        item.username,
        item.display_name,
        item.email,
        item.plan_title,
        item.plan_id,
        item.user_id,
        item.id,
      ]
        .filter((value) => value !== undefined && value !== null)
        .some((value) => String(value).toLocaleLowerCase().includes(query));
    });
  }, [keyword, statusFilter, usableItems]);

  const pagedItems = useMemo(() => {
    const start = Math.max(0, (currentPage - 1) * PAGE_SIZE);
    return filteredItems.slice(start, start + PAGE_SIZE);
  }, [currentPage, filteredItems]);

  const columns = useMemo(
    () => [
      {
        title: t('用户'),
        key: 'user',
        width: 230,
        render: (_, item) => (
          <div className='min-w-0'>
            <Text strong ellipsis={{ showTooltip: false }}>
              {item.display_name || item.username || '-'}
            </Text>
            <div className='text-xs text-gray-500'>
              @{item.username || '-'} · ID {item.user_id || '-'}
            </div>
            {item.email && (
              <div className='text-xs text-gray-500 truncate'>{item.email}</div>
            )}
          </div>
        ),
      },
      {
        title: t('套餐'),
        key: 'plan',
        width: 190,
        render: (_, item) => (
          <div className='min-w-0'>
            <Text strong ellipsis={{ showTooltip: false }}>
              {item.plan_title || `#${item.plan_id || '-'}`}
            </Text>
            <div className='text-xs text-gray-500'>
              {t('订阅实例')} #{item.id || '-'}
              {item.source === 'admin' ? ` · ${t('管理')}` : ''}
            </div>
          </div>
        ),
      },
      {
        title: t('剩余额度/总额度'),
        key: 'quota',
        width: 260,
        render: (_, item) => <QuotaCell item={item} t={t} />,
      },
      {
        title: t('有效期'),
        key: 'validity',
        width: 220,
        render: (_, item) => (
          <div className='text-xs text-gray-600'>
            <div>
              {t('结束')}: {formatTimestamp(item.end_time)}
            </div>
            <div className='text-gray-500'>
              {t('开始')}: {formatTimestamp(item.start_time)}
            </div>
          </div>
        ),
      },
      {
        title: t('状态'),
        key: 'status',
        width: 90,
        render: (_, item) => renderStatus(item, t),
      },
    ],
    [t],
  );

  return (
    <SideSheet
      visible={visible}
      placement='right'
      width={isMobile ? '100%' : 1240}
      bodyStyle={{ padding: 0 }}
      onCancel={onCancel}
      title={
        <Space>
          <IconUserGroup />
          <Typography.Title heading={4} className='m-0'>
            {t('订阅实例')}
          </Typography.Title>
        </Space>
      }
    >
      <div className='p-4'>
        <div className='grid grid-cols-1 md:grid-cols-3 gap-3 mb-4'>
          <Card className='!rounded-xl'>
            <Text type='tertiary'>{t('订阅实例')}</Text>
            <div className='text-xl font-semibold'>{usableItems.length}</div>
          </Card>
          <Card className='!rounded-xl'>
            <Text type='tertiary'>{t('生效')}</Text>
            <div className='text-xl font-semibold text-green-600'>
              {usableItems.filter((item) => item.status === 'active').length}
            </div>
          </Card>
          <Card className='!rounded-xl'>
            <Text type='tertiary'>{t('用户')}</Text>
            <div className='text-xl font-semibold'>
              {new Set(usableItems.map((item) => item.user_id)).size}
            </div>
          </Card>
        </div>

        <div className='flex flex-col md:flex-row gap-2 mb-4'>
          <div className='flex-1'>
            <Input
              prefix={<IconSearch />}
              placeholder={`${t('用户名')} / ${t('邮箱')} / ID / ${t('套餐')}`}
              value={keyword}
              onChange={setKeyword}
              showClear
              fluid
            />
          </div>
          <Select
            value={statusFilter}
            optionList={[
              { label: t('全部'), value: 'all' },
              { label: t('生效'), value: 'active' },
            ]}
            onChange={setStatusFilter}
            style={{ minWidth: isMobile ? undefined : 120 }}
            className='w-full md:w-auto'
          />
          <Button
            type='tertiary'
            icon={<IconRefresh />}
            loading={refreshing}
            onClick={() => loadSubscribers({ resetLoading: false })}
          >
            {t('刷新')}
          </Button>
        </div>

        <CardTable
          columns={columns}
          dataSource={pagedItems}
          loading={loading}
          rowKey={(row) => row?.id}
          scroll={{ x: 'max-content' }}
          pagination={{
            currentPage,
            pageSize: PAGE_SIZE,
            total: filteredItems.length,
            pageSizeOpts: [PAGE_SIZE],
            showSizeChanger: false,
            onPageChange: setCurrentPage,
          }}
          hidePagination={false}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无订阅记录')}
              style={{ padding: 30 }}
            />
          }
          size='middle'
        />
      </div>
    </SideSheet>
  );
};

export default SubscriptionSubscribersModal;
