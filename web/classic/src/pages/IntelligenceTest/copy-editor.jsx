import React, { useState } from 'react';
import {
  Button,
  Collapse,
  Space,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';

const sections = [
  {
    key: 'overview',
    title: '用户端 → 概览',
    fields: [
      ['page_title', '概览标题'],
      ['page_intro', '概览说明'],
    ],
  },
  {
    key: 'works',
    title: '用户端 → 作品与空状态',
    fields: [
      ['gallery_title', '作品说明'],
      ['empty_text', '无结果时的说明'],
    ],
  },
  {
    key: 'method',
    title: '用户端 → 测试方法',
    fields: [
      ['method_intro', '方法简介'],
      ['method_scope_title', '第一章标题'],
      ['method_evidence_title', '第二章标题'],
      ['method_selection_title', '第三章标题'],
      ['method_reading_title', '第四章标题'],
      ['method_trace_title', '第五章标题'],
      ['method_limits_title', '第六章标题'],
    ],
  },
];

export default function CopyEditor({ saved, disabled, onSave }) {
  const [open, setOpen] = useState([]);
  const [value, setValue] = useState(saved);
  return (
    <section style={{ marginBlock: 24 }}>
      <Space wrap>
        <Typography.Title heading={6}>文案管理</Typography.Title>
        <Button
          disabled={open.length === sections.length}
          onClick={() => setOpen(sections.map((s) => s.key))}
        >
          全部展开
        </Button>
        <Button disabled={!open.length} onClick={() => setOpen([])}>
          全部收起
        </Button>
      </Space>
      <Collapse activeKey={open} onChange={setOpen}>
        {sections.map((section) => {
          const changed = section.fields.some(
            ([key]) => (value[key] || '') !== (saved[key] || ''),
          );
          const count = section.fields.filter(([key]) => !!value[key]).length;
          return (
            <Collapse.Panel
              key={section.key}
              itemKey={section.key}
              header={`${section.title} · ${count} 项自定义${changed ? ' · 未保存的更改' : ''}`}
            >
              {section.fields.map(([key, label]) => (
                <div key={key} style={{ marginBottom: 16 }}>
                  <label htmlFor={`iq-copy-${key}`}>{label}</label>
                  <TextArea
                    id={`iq-copy-${key}`}
                    value={value[key] || ''}
                    disabled={disabled}
                    maxCount={500}
                    onChange={(text) =>
                      setValue((previous) => {
                        const next = { ...previous };
                        if (text) next[key] = text;
                        else delete next[key];
                        return next;
                      })
                    }
                  />
                  <Typography.Text type='tertiary'>
                    展示位置：{section.title} → {label}。留空则使用默认文案。
                  </Typography.Text>
                </div>
              ))}
            </Collapse.Panel>
          );
        })}
      </Collapse>
      <Button disabled={disabled} onClick={() => onSave(value)}>
        保存文案草稿
      </Button>
    </section>
  );
}
