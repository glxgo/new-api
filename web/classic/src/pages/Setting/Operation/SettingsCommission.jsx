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
import { Button, Form, Spin, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';

const { Text } = Typography;

const RATE_FIELDS = [
  ['普通用户邀新（直推）', 'ordinary_direct_bp'],
  ['普通用户邀新（间推）', 'ordinary_indirect_bp'],
  ['代理邀新（直推）', 'agent_direct_bp'],
  ['代理邀新（间推）', 'agent_indirect_bp'],
  ['管理员分润（直推）', 'admin_direct_bp'],
  ['管理员分润（间推）', 'admin_indirect_bp'],
  ['超级管理员分润', 'root_bp'],
];

export default function SettingsCommission() {
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [values, setValues] = useState({});
  const [original, setOriginal] = useState({});

  const load = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/profit/settings');
      if (!res.data.success || !res.data.data) {
        showError(res.data.message || '加载分润比例失败');
        return;
      }
      const next = {};
      RATE_FIELDS.forEach(([, key]) => {
        next[key] = Number(res.data.data[key] || 0) / 100;
      });
      setValues(next);
      setOriginal(next);
    } catch (error) {
      showError('加载分润比例失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const onSubmit = async () => {
    const payload = {};
    for (const [, key] of RATE_FIELDS) {
      const value = Number(values[key]);
      if (!Number.isFinite(value) || value < 0 || value > 100) {
        showError('分润比例必须在 0% 到 100% 之间');
        return;
      }
      if (value !== Number(original[key])) {
        payload[key] = Math.round(value * 100);
      }
    }
    if (Object.keys(payload).length === 0) {
      showError('你似乎并没有修改什么');
      return;
    }
    setSaving(true);
    try {
      const res = await API.put('/api/profit/settings', payload);
      if (!res.data.success) {
        showError(res.data.message || '保存分润比例失败');
        return;
      }
      showSuccess('分润比例已保存');
      await load();
    } catch (error) {
      showError('保存分润比例失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <div style={{ padding: '4px 0' }}>
        <div style={{ marginBottom: 14 }}>
          <div style={{ fontWeight: 600 }}>充值分润比例</div>
          <Text type='tertiary' style={{ fontSize: 12 }}>
            仅对后续充值结算生效，历史分润记录不会追溯修改。
          </Text>
        </div>
        <Form values={values} style={{ marginBottom: 15 }}>
          <Form.Section text='分润比例（%）'>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
                gap: 12,
              }}
            >
              {RATE_FIELDS.map(([label, key]) => (
                <Form.InputNumber
                  key={key}
                  label={label}
                  field={key}
                  min={0}
                  max={100}
                  step={0.01}
                  suffix='%'
                  value={values[key]}
                  disabled={saving}
                  onChange={(value) =>
                    setValues((prev) => ({ ...prev, [key]: value }))
                  }
                />
              ))}
            </div>
            <Button
              onClick={onSubmit}
              loading={saving}
              style={{ marginTop: 12 }}
            >
              保存分润比例
            </Button>
          </Form.Section>
        </Form>
      </div>
    </Spin>
  );
}
