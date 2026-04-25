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

import React from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Building2,
  Calculator,
  Check,
  Gauge,
  KeyRound,
  Layers,
  ReceiptText,
  ShieldCheck,
  WalletCards,
} from 'lucide-react';

const GROUP_EXAMPLES = [
  {
    group: '2折分组',
    formula: '$100 × 2折',
    pay: '$20',
    note: '适合普通按量使用',
  },
  {
    group: '1折分组',
    formula: '$100 × 1折',
    pay: '$10',
    note: '适合更高频调用',
  },
  {
    group: '0.8折分组',
    formula: '$100 × 0.8折',
    pay: '$8',
    note: '以创建 Token 时可选分组为准',
  },
];

const EXPLANATIONS = [
  {
    icon: WalletCards,
    title: '充值就是余额',
    text: '没有额外月费，账户余额可以被不同 API Token 共同使用。',
  },
  {
    icon: Layers,
    title: 'Token 决定折扣',
    text: '创建 API Token 时选择分组，之后这个 Token 的请求都按该分组计费。',
  },
  {
    icon: Gauge,
    title: '模型广场显示折扣后价格',
    text: '查看价格时看到的就是折扣后的价格，实际扣费以发起请求的 Token 分组为准。',
  },
];

const FLOW_STEPS = [
  {
    step: '01',
    title: '先充值余额',
    text: '充值金额进入账户余额，不会自动变成订阅套餐。',
  },
  {
    step: '02',
    title: '创建 API Token',
    text: '创建时选择对应分组，例如 2折、1折或更低折扣分组。',
  },
  {
    step: '03',
    title: '按 Token 自动扣费',
    text: '哪个 Token 发起请求，就按哪个 Token 的分组价格扣费。',
  },
];

const RULES = [
  '所有模型都适用，包括 GPT-5.5 和 opus-4-6。',
  '同一个账户可以创建多个不同分组的 API Token。',
  '分组以创建 API Token 页面展示为准，后台调整后新建 Token 可选择新的分组。',
  '企业客户不展示固定套餐，请直接联系人工沟通接入和额度。',
];

const PricingPlans = () => {
  const { t } = useTranslation();

  return (
    <div className='billing-static mt-[60px]'>
      <style>{`
        .billing-static {
          --bg: #f5f7fb;
          --bg-alt: #eef2f8;
          --surface: rgba(255, 255, 255, 0.78);
          --surface-strong: rgba(255, 255, 255, 0.92);
          --surface-border: rgba(15, 23, 42, 0.08);
          --text: #101828;
          --text-muted: #667085;
          --accent: #0ea5e9;
          --accent-2: #6366f1;
          --accent-3: #8b5cf6;
          --success: #16a34a;
          --warm: #f59e0b;
          --shadow-soft: 0 24px 60px rgba(15, 23, 42, 0.08);
          --shadow-strong: 0 32px 80px rgba(15, 23, 42, 0.12);
          color: var(--text);
          background:
            radial-gradient(circle at 10% 0%, rgba(14, 165, 233, 0.18), transparent 30%),
            radial-gradient(circle at 90% 8%, rgba(99, 102, 241, 0.14), transparent 28%),
            linear-gradient(180deg, var(--bg) 0%, var(--bg-alt) 100%);
          font-family:
            "Plus Jakarta Sans",
            "SF Pro Display",
            "PingFang SC",
            "Hiragino Sans GB",
            "Microsoft YaHei",
            sans-serif;
          overflow: hidden;
        }

        html.dark .billing-static {
          --bg: #080910;
          --bg-alt: #0d1019;
          --surface: rgba(255, 255, 255, 0.04);
          --surface-strong: rgba(17, 20, 38, 0.92);
          --surface-border: rgba(255, 255, 255, 0.08);
          --text: #e8eaf0;
          --text-muted: #98a2b3;
          --accent: #22d3ee;
          --accent-2: #818cf8;
          --accent-3: #a78bfa;
          --success: #4ade80;
          --warm: #fbbf24;
          --shadow-soft: 0 24px 60px rgba(0, 0, 0, 0.28);
          --shadow-strong: 0 40px 90px rgba(0, 0, 0, 0.45);
        }

        .billing-static,
        .billing-static * {
          box-sizing: border-box;
        }

        .billing-static a {
          color: inherit;
          text-decoration: none;
        }

        .billing-container {
          width: min(1180px, calc(100% - 48px));
          margin: 0 auto;
          position: relative;
          z-index: 1;
        }

        .billing-hero {
          position: relative;
          padding: 84px 0 54px;
        }

        .billing-hero::before,
        .billing-hero::after {
          content: "";
          position: absolute;
          border-radius: 999px;
          filter: blur(110px);
          pointer-events: none;
          z-index: 0;
        }

        .billing-hero::before {
          width: 360px;
          height: 360px;
          top: 26px;
          left: -80px;
          background: radial-gradient(circle, rgba(14, 165, 233, 0.18), transparent 68%);
        }

        .billing-hero::after {
          width: 420px;
          height: 420px;
          top: 32px;
          right: -90px;
          background: radial-gradient(circle, rgba(99, 102, 241, 0.14), transparent 70%);
        }

        .billing-hero-copy {
          max-width: 880px;
          margin: 0 auto;
          text-align: center;
        }

        .billing-label {
          display: inline-flex;
          align-items: center;
          gap: 10px;
          margin-bottom: 18px;
          padding: 8px 14px;
          border-radius: 999px;
          background: color-mix(in srgb, var(--accent) 10%, transparent);
          border: 1px solid color-mix(in srgb, var(--accent) 22%, transparent);
          color: var(--accent);
          font-size: 12px;
          letter-spacing: 0.12em;
          text-transform: uppercase;
          font-weight: 800;
        }

        .billing-label::before {
          content: "";
          width: 8px;
          height: 8px;
          border-radius: 999px;
          background: currentColor;
          box-shadow: 0 0 0 6px color-mix(in srgb, currentColor 18%, transparent);
        }

        .billing-title {
          margin: 0;
          color: var(--text);
          font-size: clamp(42px, 8vw, 88px);
          line-height: 1.02;
          letter-spacing: -0.06em;
          font-weight: 800;
        }

        .billing-grad-text {
          background: linear-gradient(135deg, var(--accent), var(--accent-2), var(--accent-3));
          -webkit-background-clip: text;
          background-clip: text;
          color: transparent;
        }

        .billing-subtitle {
          max-width: 760px;
          margin: 26px auto 0;
          color: var(--text-muted);
          font-size: 19px;
          line-height: 1.85;
        }

        .billing-actions {
          display: flex;
          align-items: center;
          justify-content: center;
          gap: 14px;
          margin-top: 34px;
          flex-wrap: wrap;
        }

        .billing-button-primary,
        .billing-button-secondary {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          gap: 10px;
          min-height: 52px;
          padding: 0 24px;
          border-radius: 999px;
          font-size: 15px;
          font-weight: 800;
          transition:
            transform 0.2s ease,
            box-shadow 0.2s ease,
            border-color 0.2s ease;
        }

        .billing-button-primary {
          color: #ffffff;
          background: linear-gradient(135deg, var(--accent), var(--accent-2));
          box-shadow: 0 20px 50px color-mix(in srgb, var(--accent) 28%, transparent);
        }

        .billing-button-secondary {
          color: var(--text);
          background: var(--surface);
          border: 1px solid var(--surface-border);
          backdrop-filter: blur(16px);
        }

        .billing-button-primary:hover,
        .billing-button-secondary:hover {
          transform: translateY(-2px);
        }

        .billing-spotlight {
          display: grid;
          grid-template-columns: minmax(0, 1.3fr) minmax(280px, 0.7fr);
          gap: 18px;
          align-items: stretch;
          margin-top: 44px;
          padding: 24px 26px;
          border-radius: 26px;
          background:
            linear-gradient(135deg, color-mix(in srgb, var(--accent) 10%, var(--surface-strong)) 0%, color-mix(in srgb, var(--accent-2) 9%, var(--surface)) 100%);
          border: 1px solid color-mix(in srgb, var(--accent) 22%, var(--surface-border));
          box-shadow: var(--shadow-strong);
          backdrop-filter: blur(18px);
          text-align: left;
        }

        .billing-spotlight-badge {
          display: inline-flex;
          align-items: center;
          gap: 8px;
          padding: 8px 13px;
          border-radius: 999px;
          background: color-mix(in srgb, var(--warm) 16%, transparent);
          color: var(--warm);
          font-size: 11px;
          font-weight: 800;
          letter-spacing: 0.08em;
          text-transform: uppercase;
        }

        .billing-spotlight-badge::before {
          content: "";
          width: 7px;
          height: 7px;
          border-radius: 999px;
          background: currentColor;
        }

        .billing-spotlight-title {
          margin: 14px 0 0;
          color: var(--text);
          font-size: clamp(24px, 3vw, 36px);
          line-height: 1.12;
          font-weight: 800;
          letter-spacing: -0.05em;
        }

        .billing-spotlight-text {
          margin: 12px 0 0;
          color: var(--text-muted);
          font-size: 15px;
          line-height: 1.85;
        }

        .billing-points {
          display: flex;
          flex-wrap: wrap;
          gap: 10px;
          margin-top: 18px;
        }

        .billing-point {
          display: inline-flex;
          align-items: center;
          gap: 8px;
          padding: 9px 12px;
          border-radius: 999px;
          background: color-mix(in srgb, var(--text) 5%, transparent);
          border: 1px solid var(--surface-border);
          color: var(--text);
          font-size: 12px;
          font-weight: 700;
        }

        .billing-point svg {
          color: var(--success);
        }

        .billing-formula {
          display: grid;
          gap: 10px;
          align-content: center;
          min-height: 100%;
          padding: 18px;
          border-radius: 20px;
          background: rgba(11, 17, 32, 0.94);
          color: #dbe4ff;
          box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
        }

        .billing-formula-row {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 18px;
          padding: 11px 12px;
          border-radius: 12px;
          background: rgba(255, 255, 255, 0.05);
          border: 1px solid rgba(148, 163, 184, 0.16);
          font-family: "JetBrains Mono", "SFMono-Regular", Consolas, monospace;
          font-size: 13px;
        }

        .billing-formula-row strong {
          color: #ffffff;
          font-size: 15px;
        }

        .billing-section {
          position: relative;
          padding: 66px 0;
        }

        .billing-section-tight {
          padding-top: 36px;
        }

        .billing-section-head {
          display: flex;
          align-items: flex-end;
          justify-content: space-between;
          gap: 24px;
          margin-bottom: 24px;
        }

        .billing-section-title {
          margin: 0;
          color: var(--text);
          font-size: clamp(30px, 4vw, 48px);
          line-height: 1.08;
          letter-spacing: -0.04em;
          font-weight: 800;
        }

        .billing-section-subtitle {
          max-width: 680px;
          margin: 14px 0 0;
          color: var(--text-muted);
          font-size: 16px;
          line-height: 1.8;
        }

        .billing-grid-3 {
          display: grid;
          grid-template-columns: repeat(3, minmax(0, 1fr));
          gap: 18px;
        }

        .billing-card {
          min-height: 100%;
          padding: 22px;
          border-radius: 20px;
          background: var(--surface);
          border: 1px solid var(--surface-border);
          box-shadow: var(--shadow-soft);
          backdrop-filter: blur(16px);
        }

        .billing-icon {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          width: 44px;
          height: 44px;
          margin-bottom: 18px;
          border-radius: 14px;
          color: var(--accent-2);
          background: color-mix(in srgb, var(--accent-2) 12%, transparent);
          border: 1px solid color-mix(in srgb, var(--accent-2) 18%, transparent);
        }

        .billing-card h3,
        .billing-card h4 {
          margin: 0;
          color: var(--text);
          font-weight: 800;
          letter-spacing: -0.03em;
        }

        .billing-card h3 {
          font-size: 21px;
        }

        .billing-card h4 {
          font-size: 18px;
        }

        .billing-card p {
          margin: 12px 0 0;
          color: var(--text-muted);
          font-size: 14px;
          line-height: 1.75;
        }

        .billing-example-grid {
          display: grid;
          grid-template-columns: repeat(3, minmax(0, 1fr));
          gap: 16px;
        }

        .billing-example {
          position: relative;
          overflow: hidden;
          padding: 22px;
          border-radius: 20px;
          background: var(--surface-strong);
          border: 1px solid var(--surface-border);
          box-shadow: var(--shadow-soft);
        }

        .billing-example::after {
          content: "";
          position: absolute;
          inset: 0 0 auto;
          height: 4px;
          background: linear-gradient(90deg, var(--accent), var(--accent-2), var(--accent-3));
        }

        .billing-example-name {
          color: var(--text);
          font-size: 18px;
          font-weight: 800;
        }

        .billing-example-formula {
          margin-top: 18px;
          color: var(--text-muted);
          font-family: "JetBrains Mono", "SFMono-Regular", Consolas, monospace;
          font-size: 14px;
        }

        .billing-example-pay {
          margin-top: 8px;
          color: var(--text);
          font-size: 34px;
          line-height: 1;
          letter-spacing: -0.05em;
          font-weight: 900;
        }

        .billing-example-note {
          margin-top: 14px;
          color: var(--text-muted);
          font-size: 13px;
          line-height: 1.7;
        }

        .billing-flow {
          display: grid;
          grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
          gap: 24px;
          align-items: start;
        }

        .billing-rules {
          display: grid;
          gap: 12px;
          margin-top: 24px;
        }

        .billing-rule {
          display: flex;
          align-items: flex-start;
          gap: 10px;
          color: var(--text);
          font-size: 14px;
          line-height: 1.75;
        }

        .billing-rule svg {
          flex: 0 0 auto;
          margin-top: 4px;
          color: var(--success);
        }

        .billing-step {
          display: grid;
          grid-template-columns: auto minmax(0, 1fr);
          gap: 16px;
          padding: 18px;
          border-radius: 18px;
          background: var(--surface);
          border: 1px solid var(--surface-border);
          box-shadow: var(--shadow-soft);
          backdrop-filter: blur(16px);
        }

        .billing-step + .billing-step {
          margin-top: 14px;
        }

        .billing-step-number {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          width: 44px;
          height: 44px;
          border-radius: 14px;
          color: #ffffff;
          background: linear-gradient(135deg, var(--accent), var(--accent-2));
          font-weight: 900;
        }

        .billing-step h3 {
          margin: 0;
          color: var(--text);
          font-size: 18px;
          font-weight: 800;
        }

        .billing-step p {
          margin: 8px 0 0;
          color: var(--text-muted);
          font-size: 14px;
          line-height: 1.75;
        }

        .billing-enterprise {
          display: grid;
          grid-template-columns: minmax(0, 1fr) auto;
          gap: 18px;
          align-items: center;
          margin-top: 18px;
          padding: 22px 24px;
          border-radius: 22px;
          background:
            linear-gradient(135deg, color-mix(in srgb, var(--accent-3) 14%, var(--surface-strong)) 0%, color-mix(in srgb, var(--accent) 10%, var(--surface)) 100%);
          border: 1px solid color-mix(in srgb, var(--accent-3) 28%, var(--surface-border));
          box-shadow: var(--shadow-strong);
        }

        .billing-enterprise h3 {
          display: flex;
          align-items: center;
          gap: 10px;
          margin: 0;
          color: var(--text);
          font-size: 22px;
          font-weight: 800;
          letter-spacing: -0.03em;
        }

        .billing-enterprise p {
          margin: 10px 0 0;
          color: var(--text-muted);
          font-size: 14px;
          line-height: 1.75;
        }

        .billing-enterprise-contact {
          display: flex;
          align-items: center;
          justify-content: flex-end;
          gap: 12px;
          flex-wrap: wrap;
        }

        .billing-enterprise-consult,
        .billing-enterprise-email {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          gap: 8px;
          white-space: nowrap;
          font-weight: 800;
        }

        .billing-enterprise-consult {
          min-height: 52px;
          padding: 0 22px;
          border-radius: 999px;
          color: #ffffff;
          background: linear-gradient(135deg, var(--accent-3), var(--accent-2));
          box-shadow: 0 20px 48px color-mix(in srgb, var(--accent-3) 28%, transparent);
          font-size: 15px;
        }

        .billing-enterprise-email {
          min-height: 42px;
          padding: 0 14px;
          border-radius: 999px;
          color: var(--text);
          background: color-mix(in srgb, var(--text) 5%, transparent);
          border: 1px solid var(--surface-border);
          font-size: 13px;
        }

        @media (max-width: 900px) {
          .billing-spotlight,
          .billing-flow,
          .billing-enterprise {
            grid-template-columns: 1fr;
          }

          .billing-enterprise-contact {
            justify-content: flex-start;
          }

          .billing-section-head {
            display: block;
          }

          .billing-grid-3,
          .billing-example-grid {
            grid-template-columns: 1fr;
          }
        }

        @media (max-width: 640px) {
          .billing-container {
            width: min(100% - 32px, 1180px);
          }

          .billing-hero {
            padding: 62px 0 42px;
          }

          .billing-subtitle {
            font-size: 16px;
          }

          .billing-spotlight {
            padding: 20px;
            border-radius: 22px;
          }

          .billing-actions {
            align-items: stretch;
            flex-direction: column;
          }

          .billing-button-primary,
          .billing-button-secondary {
            width: 100%;
          }

          .billing-section {
            padding: 46px 0;
          }
        }
      `}</style>

      <section className='billing-hero'>
        <div className='billing-container'>
          <div className='billing-hero-copy'>
            <div className='billing-label'>{t('按 Token 分组计费')}</div>
            <h1 className='billing-title'>
              {t('计费')}
              <span className='billing-grad-text'>{t('说明')}</span>
            </h1>
            <p className='billing-subtitle'>
              {t(
                '价格只由 API Token 绑定的分组决定。创建 Token 时选哪个分组，之后这个 Token 的请求就按对应折扣扣费。',
              )}
            </p>
            <div className='billing-actions'>
              <Link className='billing-button-primary' to='/console/token'>
                <KeyRound size={18} />
                {t('创建 API Token')}
              </Link>
              <Link className='billing-button-secondary' to='/pricing'>
                <Gauge size={18} />
                {t('查看价格')}
              </Link>
            </div>
          </div>

          <div className='billing-spotlight'>
            <div>
              <div className='billing-spotlight-badge'>{t('核心规则')}</div>
              <h2 className='billing-spotlight-title'>
                {t('模型广场显示的是折扣后的价格')}
              </h2>
              <p className='billing-spotlight-text'>
                {t(
                  '用户不需要自己再换算比例。实际调用时，系统会根据发起请求的 API Token 分组自动扣费。',
                )}
              </p>
              <div className='billing-points'>
                <span className='billing-point'>
                  <Check size={15} />
                  {t('无额外月费')}
                </span>
                <span className='billing-point'>
                  <Check size={15} />
                  {t('余额通用')}
                </span>
                <span className='billing-point'>
                  <Check size={15} />
                  {t('所有模型适用')}
                </span>
              </div>
            </div>

            <div className='billing-formula' aria-label={t('计费公式')}>
              <div className='billing-formula-row'>
                <span>{t('基础价')}</span>
                <strong>$100</strong>
              </div>
              <div className='billing-formula-row'>
                <span>{t('Token 分组')}</span>
                <strong>{t('1折')}</strong>
              </div>
              <div className='billing-formula-row'>
                <span>{t('实际扣费')}</span>
                <strong>$10</strong>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className='billing-section billing-section-tight'>
        <div className='billing-container'>
          <div className='billing-section-head'>
            <div>
              <div className='billing-label'>{t('怎么计算')}</div>
              <h2 className='billing-section-title'>{t('充值不是套餐')}</h2>
              <p className='billing-section-subtitle'>
                {t(
                  '用户充值后得到的是账户余额，不会因为充值金额自动升级。真正决定价格的是每个 API Token 创建时选择的分组。',
                )}
              </p>
            </div>
            <ShieldCheck size={38} color='var(--success)' />
          </div>

          <div className='billing-grid-3'>
            {EXPLANATIONS.map((item) => {
              const Icon = item.icon;
              return (
                <article className='billing-card' key={item.title}>
                  <span className='billing-icon'>
                    <Icon size={22} />
                  </span>
                  <h3>{t(item.title)}</h3>
                  <p>{t(item.text)}</p>
                </article>
              );
            })}
          </div>
        </div>
      </section>

      <section className='billing-section'>
        <div className='billing-container'>
          <div className='billing-section-head'>
            <div>
              <div className='billing-label'>{t('分组示例')}</div>
              <h2 className='billing-section-title'>
                {t('只看折扣，不看订阅')}
              </h2>
              <p className='billing-section-subtitle'>
                {t(
                  '下面只演示计算方式。实际可选分组和价格，以创建 API Token 页面与模型广场展示为准。',
                )}
              </p>
            </div>
            <Calculator size={38} color='var(--accent-2)' />
          </div>

          <div className='billing-example-grid'>
            {GROUP_EXAMPLES.map((row) => (
              <article className='billing-example' key={row.group}>
                <div className='billing-example-name'>{t(row.group)}</div>
                <div className='billing-example-formula'>{row.formula}</div>
                <div className='billing-example-pay'>{row.pay}</div>
                <div className='billing-example-note'>{t(row.note)}</div>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section className='billing-section'>
        <div className='billing-container billing-flow'>
          <div>
            <div className='billing-label'>{t('使用流程')}</div>
            <h2 className='billing-section-title'>{t('按 Token 分组扣费')}</h2>
            <p className='billing-section-subtitle'>
              {t(
                '这套规则避免了“充值后还要再买套餐”的困惑：余额负责支付，Token 分组负责价格。',
              )}
            </p>

            <div className='billing-rules'>
              {RULES.map((rule) => (
                <div className='billing-rule' key={rule}>
                  <Check size={17} />
                  <span>{t(rule)}</span>
                </div>
              ))}
            </div>
          </div>

          <div>
            {FLOW_STEPS.map((item) => (
              <article className='billing-step' key={item.step}>
                <span className='billing-step-number'>{item.step}</span>
                <div>
                  <h3>{t(item.title)}</h3>
                  <p>{t(item.text)}</p>
                </div>
              </article>
            ))}

            <div className='billing-enterprise'>
              <div>
                <h3>
                  <Building2 size={22} />
                  {t('企业客户')}
                </h3>
                <p>
                  {t(
                    '企业用量、专属通道、发票或采购流程不做固定价格表，请通过联系方式人工沟通。',
                  )}
                </p>
              </div>
              <div className='billing-enterprise-contact'>
                <span className='billing-enterprise-consult'>
                  <ReceiptText size={18} />
                  {t('企业服务咨询')}
                </span>
                <a
                  className='billing-enterprise-email'
                  href='mailto:lanmeimatrix@gmail.com'
                >
                  lanmeimatrix@gmail.com
                </a>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
};

export default PricingPlans;
