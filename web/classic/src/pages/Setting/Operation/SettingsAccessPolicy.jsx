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
import { Button, Modal, Spin, Switch, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

export default function SettingsAccessPolicy() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [policy, setPolicy] = useState(null);

  const load = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/access-policy');
      const { success, data, message } = res.data;
      if (success) {
        setPolicy(data);
      } else {
        showError(message || t('加载访问策略失败'));
      }
    } catch (e) {
      showError(t('加载访问策略失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const onChange = (value) => {
    Modal.confirm({
      title: value
        ? t('确认开启中国大陆访问限制？')
        : t('确认关闭中国大陆访问限制？'),
      content: value
        ? t(
            '开启后，中国大陆 IP 访问网站页面将看到 HTTP 451 提示页；API、后台管理页面和静态资源不受影响。',
          )
        : t('关闭后将移除该地区限制，请确认这是预期操作。'),
      onOk: async () => {
        setSaving(true);
        try {
          const res = await API.put('/api/access-policy', {
            block_mainland_web_access: value,
          });
          if (res.data.success) {
            showSuccess(t('保存成功'));
            await load();
          } else {
            showError(res.data.message || t('保存失败，请重试'));
          }
        } catch (e) {
          showError(t('保存失败，请重试'));
        } finally {
          setSaving(false);
        }
      },
    });
  };

  const onRollback = () => {
    Modal.confirm({
      title: t('确认恢复上一版本访问策略？'),
      content: t('当前访问策略将被上一版本替换。'),
      onOk: async () => {
        setSaving(true);
        try {
          const res = await API.post('/api/access-policy/rollback');
          if (res.data.success) {
            showSuccess(t('保存成功'));
            await load();
          } else {
            showError(res.data.message || t('回滚失败'));
          }
        } catch (e) {
          showError(t('回滚失败'));
        } finally {
          setSaving(false);
        }
      },
    });
  };

  return (
    <Spin spinning={loading} size='large'>
      {policy && (
        <div style={{ padding: '4px 0' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: '16px',
              marginBottom: '14px',
            }}
          >
            <div>
              <div style={{ fontWeight: 600 }}>{t('限制中国大陆网站访问')}</div>
              <Text type='tertiary' style={{ fontSize: 12 }}>
                {t(
                  '中国大陆 IP 访问网站页面时返回 HTTP 451；API 接口不受此开关影响。',
                )}
              </Text>
            </div>
            <Switch
              checked={policy.block_mainland_web_access}
              onChange={onChange}
              loading={saving}
            />
          </div>
          <div
            style={{
              fontSize: 13,
              color: 'rgba(var(--semi-grey-7),1)',
              lineHeight: 2,
            }}
          >
            <div>
              {t('GeoIP 数据库')}：{' '}
              {policy.geoip_db_loaded ? t('已加载') : t('未加载')}
              {policy.geoip_db_version ? ` (${policy.geoip_db_version})` : ''}
            </div>
            <div>
              {t('未知 IP 策略')}：{' '}
              {policy.geoip_unknown_policy === 'deny' ? t('拦截') : t('放行')}
            </div>
            <div>
              {t('配置版本')}：{policy.config_version}
            </div>
            <div>
              {t('已拦截请求')}：{policy.stats.block_total} ·{' '}
              {t('未知 IP 请求')}：{policy.stats.unknown_total} ·{' '}
              {t('查询错误')}：{policy.stats.lookup_error_total} ·{' '}
              {t('判定总数')}：{policy.stats.decision_total}
            </div>
          </div>
          <Button
            theme='borderless'
            type='tertiary'
            style={{ marginTop: '10px' }}
            onClick={onRollback}
            loading={saving}
          >
            {t('回滚到上一版本')}
          </Button>
        </div>
      )}
    </Spin>
  );
}
