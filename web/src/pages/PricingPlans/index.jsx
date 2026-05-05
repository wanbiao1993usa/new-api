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
  ArrowRight,
  Check,
  CreditCard,
  Infinity,
  MessageCircle,
  WalletCards,
} from 'lucide-react';

const PLANS = [
  {
    key: 'pay-as-you-go',
    name: '按量付费',
    subtitle: '按量付费，适合测试、低频使用和临时扩容',
    price: '$0',
    priceNote: '无月费',
    icon: WalletCards,
    actionText: '立即充值',
    actionTo: '/console/topup',
    features: [
      '先充值后使用',
      '相当于 0.146元/1美元',
      '按 API Token 所选分组计费',
      '余额不足时停止扣费',
      '可随时创建多个分组 Token',
    ],
  },
  {
    key: 'vip',
    name: 'VIP',
    subtitle: '大于pro x20订阅总量，相当于0.1314元/1美元',
    price: '$199',
    priceNote: '每月',
    icon: Infinity,
    actionText: '立即订阅',
    actionTo: '/console/topup?tab=subscription',
    highlighted: true,
    features: [
      '有效期: 1 个月',
      '总额度: $11111.00',
      '额度不清零',
      '升级分组: VIP',
    ],
  },
  {
    key: 'custom',
    name: '定制套餐',
    subtitle: '联系作者进行定制，适合团队和特殊用量场景',
    price: '定制',
    priceNote: '按需',
    icon: MessageCircle,
    actionText: 'lanmeimatrix@gmail.com',
    features: [
      '按业务场景配置额度',
      '可定制专属分组策略',
      '支持特殊渠道和模型需求',
      '适合团队、企业和长期用量',
    ],
  },
];

const GPT54_BASE_PRICING = {
  input: 2.5,
  output: 15,
};
const GPT54_PRICING_PROFILES = {
  payAsYouGo: {
    label: '按量付费',
    multiplier: 0.02,
  },
  vip: {
    label: 'VIP',
    multiplier: 0.018,
  },
};
const getGpt54ProfilePrice = (profile) => ({
  input: GPT54_BASE_PRICING.input * profile.multiplier,
  output: GPT54_BASE_PRICING.output * profile.multiplier,
});
const GPT54_PAY_AS_YOU_GO_PRICING = getGpt54ProfilePrice(
  GPT54_PRICING_PROFILES.payAsYouGo,
);
const GPT54_VIP_PRICING = getGpt54ProfilePrice(GPT54_PRICING_PROFILES.vip);

const PRICE_COMPARISONS = [
  {
    model: 'DeepSeek V4 Pro',
    inputPrice: 1.74,
    outputPrice: 3.48,
  },
  {
    model: 'Qwen3.6 Plus',
    inputPrice: 0.276,
    outputPrice: 1.651,
  },
  {
    model: 'GLM 5.1',
    inputPrice: 1.4,
    outputPrice: 4.4,
  },
  {
    model: 'Kimi K2.6',
    inputPrice: 0.95,
    outputPrice: 4,
  },
  {
    model: 'MiniMax M2.7',
    inputPrice: 0.3,
    outputPrice: 1.2,
  },
].map((item) => ({
  ...item,
  payAsYouGoSaving: {
    input: item.inputPrice / GPT54_PAY_AS_YOU_GO_PRICING.input,
    output: item.outputPrice / GPT54_PAY_AS_YOU_GO_PRICING.output,
  },
  vipSaving: {
    input: item.inputPrice / GPT54_VIP_PRICING.input,
    output: item.outputPrice / GPT54_VIP_PRICING.output,
  },
}));

const formatUsd = (value) =>
  `$${value > 0 && value < 0.05 ? value.toFixed(3) : value.toFixed(2)}`;
const formatSaving = (value) =>
  value >= 10 ? value.toFixed(1) : value.toFixed(2);
const formatSavingText = (value, t) => {
  if (value >= 1) {
    return t('便宜 {{times}} 倍', {
      times: formatSaving(value),
    });
  }

  return t('贵 {{times}} 倍', {
    times: formatSaving(1 / value),
  });
};
const formatProfileComparison = (saving, t) =>
  `${t('输入')} ${formatSavingText(saving.input, t)} / ${t('输出')} ${formatSavingText(
    saving.output,
    t,
  )}`;

const PricingPlans = () => {
  const { t } = useTranslation();

  return (
    <main className='plans-page mt-[60px]'>
      <style>{`
        .plans-page {
          --plans-bg: #f7f8fb;
          --plans-surface: #ffffff;
          --plans-surface-muted: #f2f4f7;
          --plans-border: #e4e7ec;
          --plans-border-strong: #cfd5df;
          --plans-text: #101828;
          --plans-muted: #667085;
          --plans-primary: #1f6feb;
          --plans-primary-strong: #175cd3;
          --plans-success: #16a34a;
          --plans-shadow: 0 18px 42px rgba(16, 24, 40, 0.08);
          min-height: calc(100vh - 60px);
          background: var(--plans-bg);
          color: var(--plans-text);
          font-family:
            "Inter",
            "SF Pro Display",
            "PingFang SC",
            "Hiragino Sans GB",
            "Microsoft YaHei",
            sans-serif;
        }

        html.dark .plans-page {
          --plans-bg: #0b0d12;
          --plans-surface: #111827;
          --plans-surface-muted: #1f2937;
          --plans-border: #2f3847;
          --plans-border-strong: #475467;
          --plans-text: #f2f4f7;
          --plans-muted: #98a2b3;
          --plans-primary: #60a5fa;
          --plans-primary-strong: #93c5fd;
          --plans-success: #4ade80;
          --plans-shadow: 0 18px 42px rgba(0, 0, 0, 0.32);
        }

        .plans-page,
        .plans-page * {
          box-sizing: border-box;
        }

        .plans-wrap {
          width: min(1120px, calc(100% - 48px));
          margin: 0 auto;
          padding: 56px 0 72px;
        }

        .plans-heading {
          display: grid;
          grid-template-columns: minmax(0, 1fr) auto;
          gap: 24px;
          align-items: end;
          margin-bottom: 28px;
        }

        .plans-kicker {
          display: flex;
          align-items: center;
          gap: 8px;
          margin: 0 0 10px;
          color: var(--plans-primary);
          font-size: 14px;
          font-weight: 700;
          letter-spacing: 0;
        }

        .plans-kicker svg {
          flex: 0 0 auto;
        }

        .plans-title {
          margin: 0;
          color: var(--plans-text);
          font-size: clamp(34px, 5vw, 56px);
          line-height: 1.08;
          font-weight: 800;
          letter-spacing: 0;
        }

        .plans-description {
          max-width: 720px;
          margin: 14px 0 0;
          color: var(--plans-muted);
          font-size: 16px;
          line-height: 1.75;
        }

        .plans-model-link {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          gap: 8px;
          min-height: 40px;
          padding: 0 14px;
          border-radius: 8px;
          color: var(--plans-text);
          background: var(--plans-surface);
          border: 1px solid var(--plans-border);
          font-size: 14px;
          font-weight: 700;
          text-decoration: none;
          transition:
            border-color 0.18s ease,
            color 0.18s ease,
            transform 0.18s ease;
        }

        .plans-model-link:hover {
          color: var(--plans-primary);
          border-color: var(--plans-primary);
          transform: translateY(-1px);
        }

        .plans-grid {
          display: grid;
          grid-template-columns: repeat(3, minmax(0, 1fr));
          gap: 20px;
          align-items: stretch;
        }

        .plan-card {
          position: relative;
          display: flex;
          flex-direction: column;
          min-height: 520px;
          padding: 32px;
          border-radius: 8px;
          background: var(--plans-surface);
          border: 1px solid var(--plans-border);
          box-shadow: var(--plans-shadow);
        }

        .plan-card-highlighted {
          border-color: var(--plans-border-strong);
        }

        .plan-badge {
          position: absolute;
          top: 18px;
          right: 18px;
          display: inline-flex;
          align-items: center;
          gap: 6px;
          padding: 6px 10px;
          border-radius: 8px;
          color: var(--plans-primary-strong);
          background: color-mix(in srgb, var(--plans-primary) 10%, transparent);
          border: 1px solid color-mix(in srgb, var(--plans-primary) 28%, transparent);
          font-size: 12px;
          font-weight: 700;
        }

        .plan-icon {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          width: 44px;
          height: 44px;
          margin-bottom: 22px;
          border-radius: 8px;
          color: var(--plans-primary);
          background: color-mix(in srgb, var(--plans-primary) 10%, transparent);
        }

        .plan-name {
          margin: 0;
          color: var(--plans-text);
          font-size: clamp(28px, 4vw, 36px);
          line-height: 1.15;
          font-weight: 800;
          letter-spacing: 0;
        }

        .plan-subtitle {
          min-height: 58px;
          margin: 10px 0 0;
          color: var(--plans-muted);
          font-size: 18px;
          line-height: 1.55;
        }

        .plan-price-row {
          display: flex;
          align-items: baseline;
          gap: 10px;
          margin: 42px 0 24px;
        }

        .plan-price {
          color: var(--plans-text);
          font-size: clamp(42px, 5vw, 60px);
          line-height: 1;
          font-weight: 800;
          letter-spacing: 0;
        }

        .plan-price-note {
          color: var(--plans-muted);
          font-size: 16px;
          font-weight: 700;
        }

        .plan-features {
          display: grid;
          gap: 12px;
          margin: 0 0 30px;
          padding: 0;
          list-style: none;
        }

        .plan-feature {
          display: grid;
          grid-template-columns: 20px minmax(0, 1fr);
          gap: 10px;
          align-items: start;
          color: var(--plans-text);
          font-size: 17px;
          line-height: 1.55;
        }

        .plan-feature svg {
          margin-top: 4px;
          color: var(--plans-success);
        }

        .plan-action {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          gap: 8px;
          width: 100%;
          min-height: 48px;
          margin-top: auto;
          border-radius: 8px;
          color: #ffffff;
          background: var(--plans-primary);
          font-size: 15px;
          font-weight: 800;
          text-decoration: none;
          transition:
            background 0.18s ease,
            transform 0.18s ease;
        }

        .plan-action:hover {
          background: var(--plans-primary-strong);
          transform: translateY(-1px);
        }

        .plan-action-static,
        .plan-action-static:hover {
          cursor: default;
          background: var(--plans-primary);
          transform: none;
        }

        .plans-note {
          margin: 20px 0 0;
          padding: 16px 18px;
          border-radius: 8px;
          color: var(--plans-muted);
          background: var(--plans-surface);
          border: 1px solid var(--plans-border);
          font-size: 14px;
          line-height: 1.7;
        }

        .plans-comparison {
          margin-top: 24px;
          padding: 28px;
          border-radius: 8px;
          background: var(--plans-surface);
          border: 1px solid var(--plans-border);
          box-shadow: var(--plans-shadow);
        }

        .plans-comparison-header {
          display: grid;
          grid-template-columns: minmax(0, 1fr) auto;
          gap: 20px;
          align-items: end;
          margin-bottom: 18px;
        }

        .plans-comparison-title {
          margin: 0;
          color: var(--plans-text);
          font-size: 24px;
          line-height: 1.25;
          font-weight: 800;
          letter-spacing: 0;
        }

        .plans-comparison-desc {
          max-width: 760px;
          margin: 8px 0 0;
          color: var(--plans-muted);
          font-size: 14px;
          line-height: 1.7;
        }

        .plans-comparison-benchmark {
          display: grid;
          gap: 4px;
          min-width: 190px;
          padding: 12px 14px;
          border-radius: 8px;
          background: var(--plans-surface-muted);
          color: var(--plans-text);
          font-size: 14px;
          font-weight: 700;
          text-align: right;
        }

        .plans-comparison-benchmark span {
          color: var(--plans-muted);
          font-size: 12px;
          font-weight: 600;
        }

        .plans-table-wrap {
          overflow-x: auto;
          border: 1px solid var(--plans-border);
          border-radius: 8px;
        }

        .plans-table {
          width: 100%;
          min-width: 720px;
          border-collapse: collapse;
          font-size: 14px;
        }

        .plans-table th,
        .plans-table td {
          padding: 15px 16px;
          border-bottom: 1px solid var(--plans-border);
          text-align: left;
          white-space: nowrap;
        }

        .plans-table th {
          color: var(--plans-muted);
          background: var(--plans-surface-muted);
          font-size: 12px;
          font-weight: 800;
        }

        .plans-table tbody tr:last-child td {
          border-bottom: 0;
        }

        .plans-table-model {
          color: var(--plans-text);
          font-weight: 800;
        }

        .plans-table-price {
          color: var(--plans-text);
          font-variant-numeric: tabular-nums;
          font-weight: 700;
        }

        .plans-table-saving {
          color: var(--plans-primary);
          font-variant-numeric: tabular-nums;
          font-weight: 800;
        }

        .plans-comparison-footnote {
          margin: 14px 0 0;
          color: var(--plans-muted);
          font-size: 12px;
          line-height: 1.6;
        }

        @media (max-width: 860px) {
          .plans-heading {
            grid-template-columns: 1fr;
            align-items: start;
          }

          .plans-grid {
            grid-template-columns: 1fr;
          }

          .plans-comparison-header {
            grid-template-columns: 1fr;
            align-items: start;
          }

          .plans-comparison-benchmark {
            text-align: left;
          }

          .plan-card {
            min-height: auto;
          }
        }

        @media (max-width: 640px) {
          .plans-wrap {
            width: min(100% - 28px, 1120px);
            padding: 36px 0 48px;
          }

          .plan-card {
            padding: 24px;
          }

          .plans-comparison {
            padding: 20px;
          }

          .plan-subtitle {
            min-height: 0;
            font-size: 16px;
          }

          .plan-price-row {
            margin: 30px 0 22px;
          }

          .plan-feature {
            font-size: 16px;
          }
        }
      `}</style>

      <div className='plans-wrap'>
        <header className='plans-heading'>
          <div>
            <p className='plans-kicker'>
              <CreditCard size={16} />
              {t('价格方案')}
            </p>
            <h1 className='plans-title'>{t('选择适合你的价格方案')}</h1>
            <p className='plans-description'>
              {t(
                '低频调用可以按量付费；重度使用可以选择 VIP 套餐，按固定周期获得更高额度。',
              )}
            </p>
          </div>
          <Link className='plans-model-link' to='/pricing'>
            {t('查看模型价格')}
            <ArrowRight size={16} />
          </Link>
        </header>

        <section className='plans-grid' aria-label={t('价格方案')}>
          {PLANS.map((plan) => {
            const Icon = plan.icon;
            return (
              <article
                className={`plan-card ${
                  plan.highlighted ? 'plan-card-highlighted' : ''
                }`}
                key={plan.key}
              >
                {plan.highlighted && (
                  <div className='plan-badge'>
                    <Check size={13} />
                    {t('推荐')}
                  </div>
                )}

                <div className='plan-icon'>
                  <Icon size={24} />
                </div>
                <h2 className='plan-name'>{t(plan.name)}</h2>
                <p className='plan-subtitle'>{t(plan.subtitle)}</p>

                <div className='plan-price-row'>
                  <span className='plan-price'>{plan.price}</span>
                  <span className='plan-price-note'>{t(plan.priceNote)}</span>
                </div>

                <ul className='plan-features'>
                  {plan.features.map((feature) => (
                    <li className='plan-feature' key={feature}>
                      <Check size={17} />
                      <span>{t(feature)}</span>
                    </li>
                  ))}
                </ul>

                {plan.actionTo ? (
                  <Link className='plan-action' to={plan.actionTo}>
                    {t(plan.actionText)}
                    <ArrowRight size={16} />
                  </Link>
                ) : (
                  <span className='plan-action plan-action-static'>
                    {t(plan.actionText)}
                  </span>
                )}
              </article>
            );
          })}
        </section>

        <section
          className='plans-comparison'
          aria-label={t('顶级模型价格对比')}
        >
          <div className='plans-comparison-header'>
            <div>
              <h2 className='plans-comparison-title'>
                {t('顶级模型价格对比')}
              </h2>
              <p className='plans-comparison-desc'>
                {t(
                  '以 GPT-5.4 为基准，分别按按量付费倍率和 VIP 倍率，对比国内顶级大模型原价，直接展示便宜多少倍。',
                )}
              </p>
            </div>
            <div className='plans-comparison-benchmark'>
              <span>{t('GPT-5.4 基准价')}</span>
              {t('按量付费')}: {formatUsd(GPT54_PAY_AS_YOU_GO_PRICING.input)} /{' '}
              {formatUsd(GPT54_PAY_AS_YOU_GO_PRICING.output)}
              <br />
              {t('VIP')}: {formatUsd(GPT54_VIP_PRICING.input)} /{' '}
              {formatUsd(GPT54_VIP_PRICING.output)}
            </div>
          </div>

          <div className='plans-table-wrap'>
            <table className='plans-table'>
              <thead>
                <tr>
                  <th>{t('对比模型')}</th>
                  <th>{t('国内模型原价 / 1M tokens')}</th>
                  <th>{t('按量付费')}</th>
                  <th>{t('VIP')}</th>
                </tr>
              </thead>
              <tbody>
                {PRICE_COMPARISONS.map((item) => (
                  <tr key={item.model}>
                    <td className='plans-table-model'>{t(item.model)}</td>
                    <td className='plans-table-price'>
                      {t('输入')} {formatUsd(item.inputPrice)} / {t('输出')}{' '}
                      {formatUsd(item.outputPrice)}
                    </td>
                    <td className='plans-table-saving'>
                      {formatProfileComparison(item.payAsYouGoSaving, t)}
                    </td>
                    <td className='plans-table-saving'>
                      {formatProfileComparison(item.vipSaving, t)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <p className='plans-comparison-footnote'>
            {t(
              '注：表内价格单位为美元 / 1M tokens；按量付费按 GPT-5.4 标准价 2% 计算，VIP 按 GPT-5.4 标准价 1.8% 计算。',
            )}
          </p>
        </section>

        <p className='plans-note'>
          {t(
            '实际扣费以 API Token 所选分组、模型价格和订阅扣费策略为准。套餐购买入口会展示当前可购买的实时套餐配置。',
          )}
        </p>
      </div>
    </main>
  );
};

export default PricingPlans;
