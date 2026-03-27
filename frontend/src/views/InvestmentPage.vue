<template>
  <div class="section-header">
    <div>
      <h1>投资批次</h1>
      <div class="light-text">选择买入批次，计算均价并快速生成卖出记录</div>
    </div>
    <div class="toolbar toolbar-investments">
      <div class="toolbar-filters">
        <el-select v-model="statusFilter" placeholder="状态" style="min-width: 140px">
          <el-option label="全部" value="all" />
          <el-option label="未匹配" value="open" />
          <el-option label="已匹配" value="closed" />
        </el-select>
        <el-select v-model="tagFilter" clearable placeholder="标签" style="min-width: 140px">
          <el-option label="建仓" value="建仓" />
          <el-option label="定投" value="定投" />
          <el-option label="打野" value="打野" />
        </el-select>
        <el-select v-model="cashAccountFilter" clearable placeholder="资金账户" style="min-width: 160px">
          <el-option v-for="account in brokerCashAccounts" :key="account.id" :label="account.name" :value="account.id" />
        </el-select>
        <el-date-picker
          v-model="buyDateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="买入开始日期"
          end-placeholder="买入结束日期"
          :shortcuts="buyDateShortcuts"
          value-format="YYYY-MM-DD"
          style="min-width: 340px"
          @change="reloadLots"
        />
        <el-input v-model="keyword" placeholder="搜索标的" clearable style="min-width: 200px" />
      </div>
      <div class="toolbar-actions">
        <el-button type="primary" plain @click="openBuyDialog">新增买入</el-button>
        <el-button type="primary" plain :loading="priceRefreshing" @click="refreshPrices">更新现价</el-button>
        <el-button type="primary" plain @click="openStrategyDialog">策略模板</el-button>
        <el-button type="primary" plain @click="openTransferDialog">银证转账</el-button>
        <el-button type="primary" :disabled="selectedLots.length === 0" @click="openSellDialog">批次卖出</el-button>
        <el-button :icon="RefreshRight" :loading="loading" @click="loadLots">刷新</el-button>
      </div>
    </div>
  </div>

  <div class="card">
    <div class="table-scroll">
      <el-table
        :data="filteredLots"
        stripe
        border
        v-loading="loading"
        style="width: 100%; min-width: 1400px"
        row-key="lotId"
        show-summary
        :summary-method="summaryMethod"
        :row-class-name="rowClassName"
        @cell-mouse-enter="onCellMouseEnter"
        @cell-mouse-leave="onCellMouseLeave"
        @selection-change="onSelectionChange"
      >
      <el-table-column type="selection" width="52" :selectable="isSelectable" />
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="row.status === 'open' ? 'success' : 'info'" effect="light">
            {{ row.status === 'open' ? '未匹配' : '已匹配' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="标的代码" min-width="120">
        <template #default="{ row }">
          <span class="security-ticker">{{ row.securityTicker }}</span>
        </template>
      </el-table-column>
      <el-table-column label="标的名称" min-width="160">
        <template #default="{ row }">
          <span class="security-name">{{ row.securityName }}</span>
        </template>
      </el-table-column>
      <el-table-column label="标签" width="100">
        <template #default="{ row }">
          <span>{{ row.tag || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="occurredOn" label="买入日期" width="140" />
      <el-table-column label="成交价" width="130" align="right">
        <template #default="{ row }">
          {{ formatNumber(row.tradePrice > 0 ? row.tradePrice : row.price, 3) }}
        </template>
      </el-table-column>
      <el-table-column label="成本价" width="130" align="right">
        <template #default="{ row }">
          <span>{{ formatNumber(row.price, 3) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="现价" width="130" align="right">
        <template #default="{ row }">
          {{ row.currentPrice > 0 ? formatNumber(row.currentPrice, 3) : '-' }}
        </template>
      </el-table-column>
      <el-table-column label="手续费" width="120" align="right">
        <template #default="{ row }">{{ formatNumber(row.fee, 3) }}</template>
      </el-table-column>
      <!-- 税费暂时停用 -->
      <!-- <el-table-column label="税费" width="120" align="right">
        <template #default="{ row }">{{ formatNumber(row.tax, 3) }}</template>
      </el-table-column> -->
      <el-table-column label="总数量" width="130" align="right">
        <template #default="{ row }">{{ formatNumber(row.quantity, 0) }}</template>
      </el-table-column>
      <el-table-column label="盈利金额" width="140" align="right">
        <template #default="{ row }">
          <span v-if="profitAmount(row) !== null" :class="['profit-cell', profitClass(profitAmount(row) || 0)]">
            {{ formatNumber(profitAmount(row) || 0, 4) }}
          </span>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column label="盈利点" width="120" align="right">
        <template #default="{ row }">
          <span v-if="profitPct(row) !== null" :class="['profit-cell', profitClass(profitPct(row) || 0)]">
            {{ formatPercent(profitPct(row) || 0) }}
          </span>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column label="已匹配" width="130" align="right">
        <template #default="{ row }">{{ formatNumber(row.allocatedQuantity, 0) }}</template>
      </el-table-column>
      <el-table-column label="可卖数量" width="130" align="right">
        <template #default="{ row }">{{ formatNumber(Math.max(row.remainingQuantity, 0), 0) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="240" align="right">
        <template #default="{ row }">
          <div class="table-actions">
            <el-button size="small" @click="openBuyDialogFromRow(row)">快速添加</el-button>
            <el-button size="small" type="primary" plain :disabled="row.allocatedQuantity > 0" @click="openEditBuyDialog(row)">
              修改
            </el-button>
            <el-button
              size="small"
              type="danger"
              plain
              :disabled="row.allocatedQuantity > 0"
              :loading="deletingLotId === row.lotId"
              @click="confirmDeleteLot(row)"
            >
              删除
            </el-button>
          </div>
        </template>
      </el-table-column>
      </el-table>

      <el-tooltip
        v-if="hoveredIndicatorRow"
        :virtual-ref="hoveredCellRef"
        virtual-triggering
        :visible="indicatorTooltipVisible"
        effect="dark"
        placement="top"
      >
        <template #content>
          <div class="indicator-tooltip">
            <div>5日均线: {{ formatIndicator(hoveredIndicatorRow.ma5) }}</div>
            <div>55日最高: {{ formatIndicator(hoveredIndicatorRow.high55) }}</div>
            <div>20日最高: {{ formatIndicator(hoveredIndicatorRow.high20) }}</div>
            <div>10日最低: {{ formatIndicator(hoveredIndicatorRow.low10) }}</div>
            <div>20日最低: {{ formatIndicator(hoveredIndicatorRow.low20) }}</div>
          </div>
        </template>
      </el-tooltip>
    </div>

    <div class="pagination-row">
      <el-pagination
        background
        layout="prev, pager, next, sizes, total"
        :page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :current-page="page"
        :total="total"
        @size-change="onPageSizeChange"
        @current-change="onPageChange"
      />
    </div>

    <div class="overall-summary">
      <div class="overall-item">
        <span class="overall-label">资金余额</span>
        <span class="overall-value">
          {{ cashBalanceLoading ? '...' : cashBalance === null ? '-' : formatNumber(cashBalance, 2) }}
        </span>
      </div>
      <div class="overall-item">
        <span class="overall-label">持仓成本</span>
        <span class="overall-value">
          {{ formatNumber(overallSummary.totalCost, 2) }}
        </span>
      </div>
      <div class="overall-item">
        <span class="overall-label">持仓市值</span>
        <span class="overall-value">
          {{ overallSummary.hasMarket ? formatNumber(overallSummary.totalMarket, 2) : '-' }}
        </span>
      </div>
      <div class="overall-item">
        <span class="overall-label">总资产(成本)</span>
        <span class="overall-value">
          {{ totalAssetCost === null ? '-' : formatNumber(totalAssetCost, 2) }}
        </span>
      </div>
      <div class="overall-item">
        <span class="overall-label">总资产(市值)</span>
        <span class="overall-value">
          {{ totalAssetMarket === null ? '-' : formatNumber(totalAssetMarket, 2) }}
        </span>
      </div>
      <div class="overall-item">
        <span class="overall-label">盈亏</span>
        <span
          class="overall-value"
          :class="overallSummary.hasMarket ? profitClass(overallSummary.profit) : 'profit-neutral'"
        >
          {{ overallSummary.hasMarket ? formatNumber(overallSummary.profit, 2) : '-' }}
        </span>
      </div>
      <div class="overall-note">资金余额统计日期 {{ cashBalanceAsOf }}</div>
      <div v-if="overallSummary.partialMarket" class="overall-note">仅统计有现价的批次</div>
    </div>
  </div>

  <el-dialog v-model="dialogVisible" title="批次卖出" width="960px" destroy-on-close>
    <div class="dialog-summary">
      <div>
        <div class="summary-label">标的</div>
        <div class="summary-value">{{ selectedSecurityLabel }}</div>
      </div>
      <div>
        <div class="summary-label">总数量</div>
        <div class="summary-value">{{ formatNumber(totalQuantity, 0) }}</div>
      </div>
      <div>
        <div class="summary-label">平均成本</div>
        <div class="summary-value">{{ formatNumber(avgCost, 3) }}</div>
      </div>
      <div>
        <div class="summary-label">目标价</div>
        <div class="summary-value">{{ formatNumber(targetPrice, 3) }}</div>
      </div>
    </div>

    <el-table :data="allocationRows" size="small" border style="margin-bottom: 16px">
      <el-table-column label="批次" min-width="180">
        <template #default="{ row }">
          <div class="security-cell">
            <div class="security-ticker">{{ row.securityTicker }}</div>
            <div class="security-name">{{ row.occurredOn }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="成本价" width="120" align="right">
        <template #default="{ row }">{{ formatNumber(row.price, 3) }}</template>
      </el-table-column>
      <el-table-column label="可卖数量" width="140" align="right">
        <template #default="{ row }">{{ formatNumber(row.remainingQuantity, 0) }}</template>
      </el-table-column>
      <el-table-column label="卖出数量" width="180">
        <template #default="{ row }">
          <el-input-number
            v-model="row.quantity"
            :min="0"
            :max="row.remainingQuantity"
            :step="1"
            :precision="0"
            controls-position="right"
          />
        </template>
      </el-table-column>
    </el-table>

    <el-form label-width="120px" class="dialog-form">
      <el-form-item label="卖出日期">
        <el-date-picker v-model="saleForm.occurredOn" type="date" value-format="YYYY-MM-DD" />
      </el-form-item>
      <el-form-item label="卖出价格">
        <el-input-number v-model="saleForm.price" :min="0" :step="0.001" :precision="3" controls-position="right" />
      </el-form-item>
      <el-form-item label="目标盈利(%)">
        <div class="inline-row">
          <el-input-number
            v-model="saleForm.targetProfitPct"
            :min="1"
            :max="100"
            :step="0.1"
            :precision="2"
            controls-position="right"
          />
          <span class="hint-text">目标价 {{ formatNumber(targetPrice, 3) }}</span>
        </div>
      </el-form-item>
      <el-form-item label="资金账户">
        <template v-if="brokerCashAccounts.length > 1">
          <el-select v-model="saleForm.cashAccountId" placeholder="选择证券资金账户" style="width: 100%">
            <el-option
              v-for="account in brokerCashAccounts"
              :key="account.id"
              :label="account.name"
              :value="account.id"
            />
          </el-select>
        </template>
        <template v-else>
          <el-input
            :model-value="brokerCashAccounts[0]?.name || '未配置证券资金账户'"
            placeholder="未配置证券资金账户"
            disabled
          />
        </template>
      </el-form-item>
      <el-form-item label="持仓账户">
        <template v-if="investmentAccounts.length > 1">
          <el-select v-model="saleForm.investmentAccountId" placeholder="选择持仓账户" style="width: 100%">
            <el-option
              v-for="account in investmentAccounts"
              :key="account.id"
              :label="account.name"
              :value="account.id"
            />
          </el-select>
        </template>
        <template v-else>
          <el-input
            :model-value="investmentAccounts[0]?.name || '未配置持仓账户'"
            placeholder="未配置持仓账户"
            disabled
          />
        </template>
      </el-form-item>
      <el-form-item label="手续费">
        <div class="inline-row">
          <el-input-number v-model="saleForm.fee" :min="0" :step="0.001" :precision="3" controls-position="right" />
          <el-select v-model="saleForm.feeCategoryId" clearable placeholder="费用分类" style="min-width: 180px">
            <el-option
              v-for="category in expenseCategories"
              :key="category.id"
              :label="category.name"
              :value="category.id"
            />
          </el-select>
        </div>
      </el-form-item>
      <!-- 税费暂时停用 -->
      <!-- <el-form-item label="税费">
        <div class="inline-row">
          <el-input-number v-model="saleForm.tax" :min="0" :step="0.001" :precision="3" controls-position="right" />
          <el-select v-model="saleForm.taxCategoryId" clearable placeholder="税费分类" style="min-width: 180px">
            <el-option
              v-for="category in expenseCategories"
              :key="category.id"
              :label="category.name"
              :value="category.id"
            />
          </el-select>
        </div>
      </el-form-item> -->
      <el-form-item label="备注">
        <el-input v-model="saleForm.description" placeholder="可选：卖出说明" />
      </el-form-item>
      <el-form-item label="盈利预估">
        <div class="summary-inline">
          <span>毛收入 {{ formatNumber(grossAmount, 2) }}</span>
          <span>成本 {{ formatNumber(totalCost, 2) }}</span>
          <span>费用 {{ formatNumber(totalFeeAndTax, 2) }}</span>
          <span :class="profitPreview >= 0 ? 'profit-positive' : 'profit-negative'">
            预计盈亏 {{ formatNumber(profitPreview, 2) }}
          </span>
        </div>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="submitSale">确认卖出</el-button>
    </template>
  </el-dialog>

  <el-dialog
    v-model="buyDialogVisible"
    :title="buyMode === 'edit' ? '修改买入' : '新增买入'"
    width="720px"
    destroy-on-close
  >
    <div class="dialog-summary">
      <div>
        <div class="summary-label">成交额</div>
        <div class="summary-value">{{ formatNumber(buyGrossAmount, 2) }}</div>
      </div>
      <div>
        <div class="summary-label">含费成本</div>
        <div class="summary-value">{{ formatNumber(buyCostAmount, 2) }}</div>
      </div>
      <div>
        <div class="summary-label">成本价</div>
        <div class="summary-value">{{ formatNumber(buyCostPrice, 3) }}</div>
      </div>
      <div>
        <div class="summary-label">手续费</div>
        <div class="summary-value">{{ formatNumber(buyFeeAndTax, 2) }}</div>
      </div>
    </div>

    <el-form label-width="120px" class="dialog-form">
      <el-form-item label="买入日期">
        <el-date-picker v-model="buyForm.occurredOn" type="date" value-format="YYYY-MM-DD" />
      </el-form-item>
      <el-form-item label="标的代码">
        <el-input v-model="buyForm.securityTicker" placeholder="例如 AAPL / 600519" />
      </el-form-item>
      <el-form-item label="标的名称">
        <el-input v-model="buyForm.securityName" placeholder="例如 Apple / 贵州茅台" />
      </el-form-item>
      <el-form-item label="标签">
        <el-select v-model="buyForm.tag" clearable placeholder="选择标签" style="width: 100%">
          <el-option label="建仓" value="建仓" />
          <el-option label="定投" value="定投" />
          <el-option label="打野" value="打野" />
        </el-select>
      </el-form-item>
      <el-form-item label="买入数量">
        <el-input-number v-model="buyForm.quantity" :min="0" :step="1" :precision="0" controls-position="right" />
      </el-form-item>
      <el-form-item label="成交价">
        <el-input-number v-model="buyForm.price" :min="0" :step="0.001" :precision="3" controls-position="right" />
      </el-form-item>
      <el-form-item label="资金账户">
        <template v-if="brokerCashAccounts.length > 1">
          <el-select v-model="buyForm.cashAccountId" placeholder="选择证券资金账户" style="width: 100%">
            <el-option
              v-for="account in brokerCashAccounts"
              :key="account.id"
              :label="account.name"
              :value="account.id"
            />
          </el-select>
        </template>
        <template v-else>
          <el-input
            :model-value="brokerCashAccounts[0]?.name || '未配置证券资金账户'"
            placeholder="未配置证券资金账户"
            disabled
          />
        </template>
      </el-form-item>
      <el-form-item label="持仓账户">
        <template v-if="investmentAccounts.length > 1">
          <el-select v-model="buyForm.investmentAccountId" placeholder="选择持仓账户" style="width: 100%">
            <el-option
              v-for="account in investmentAccounts"
              :key="account.id"
              :label="account.name"
              :value="account.id"
            />
          </el-select>
        </template>
        <template v-else>
          <el-input
            :model-value="investmentAccounts[0]?.name || '未配置持仓账户'"
            placeholder="未配置持仓账户"
            disabled
          />
        </template>
      </el-form-item>
      <el-form-item label="手续费">
        <div class="inline-row">
          <el-input-number v-model="buyForm.fee" :min="0" :step="0.001" :precision="3" controls-position="right" />
          <el-select v-model="buyForm.feeCategoryId" clearable placeholder="费用分类" style="min-width: 180px">
            <el-option
              v-for="category in expenseCategories"
              :key="category.id"
              :label="category.name"
              :value="category.id"
            />
          </el-select>
        </div>
      </el-form-item>
      <!-- 税费暂时停用 -->
      <!-- <el-form-item label="税费">
        <div class="inline-row">
          <el-input-number v-model="buyForm.tax" :min="0" :step="0.001" :precision="3" controls-position="right" />
          <el-select v-model="buyForm.taxCategoryId" clearable placeholder="税费分类" style="min-width: 180px">
            <el-option
              v-for="category in expenseCategories"
              :key="category.id"
              :label="category.name"
              :value="category.id"
            />
          </el-select>
        </div>
      </el-form-item> -->
      <el-form-item label="备注">
        <el-input v-model="buyForm.description" placeholder="可选：买入说明" />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="buyDialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="buySaving" @click="submitBuy">
        {{ buyMode === 'edit' ? '保存修改' : '保存买入' }}
      </el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="transferDialogVisible" title="银证转账" width="560px" destroy-on-close>
    <el-form label-width="100px">
      <el-form-item label="转账方向">
        <el-radio-group v-model="transferForm.direction">
          <el-radio-button label="bank_to_broker">银行转证券</el-radio-button>
          <el-radio-button label="broker_to_bank">证券转银行</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="转账日期">
        <el-date-picker v-model="transferForm.occurredOn" type="date" value-format="YYYY-MM-DD" />
      </el-form-item>
      <el-form-item label="转出账户">
        <el-select v-model="transferForm.fromAccountId" placeholder="选择转出账户" style="width: 100%">
          <el-option
            v-for="account in transferFromAccounts"
            :key="account.id"
            :label="account.name"
            :value="account.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="转入账户">
        <el-select v-model="transferForm.toAccountId" placeholder="选择转入账户" style="width: 100%">
          <el-option
            v-for="account in transferToAccounts"
            :key="account.id"
            :label="account.name"
            :value="account.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="转账金额">
        <el-input-number v-model="transferForm.amount" :min="0" :step="0.01" :precision="2" controls-position="right" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="transferForm.description" placeholder="可选：银证转账说明" />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="transferDialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="transferSaving" @click="submitTransfer">确认转账</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="strategyDialogVisible" title="策略模板" width="860px" destroy-on-close>
    <el-table :data="strategies" border stripe v-loading="strategyLoading" style="width: 100%">
      <el-table-column label="名称" min-width="180">
        <template #default="{ row }">
          <div class="strategy-name">{{ row.name }}</div>
          <div class="strategy-kind">{{ row.kind || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.isActive ? 'success' : 'info'" effect="light">
            {{ row.isActive ? '启用' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="概要" min-width="360">
        <template #default="{ row }">
          <div class="strategy-summary">
            <div>入场：{{ strategyEntrySummary(row) }}</div>
            <div>止损/止盈：{{ strategyExitSummary(row) }}</div>
            <div>周期：{{ strategyProfileSummary(row) }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="详情" width="100">
        <template #default="{ row }">
          <el-popover placement="left" width="360" trigger="hover">
            <template #reference>
              <el-button size="small">查看</el-button>
            </template>
            <pre class="strategy-json">{{ formatStrategyParams(row) }}</pre>
          </el-popover>
        </template>
      </el-table-column>
    </el-table>

    <template #footer>
      <el-button @click="strategyDialogVisible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { RefreshRight } from '@element-plus/icons-vue';
import {
  fetchInvestmentLots,
  createInvestmentBuy,
  createInvestmentSale,
  updateInvestmentBuy,
  deleteInvestmentBuy,
  refreshInvestmentPrices,
  refreshInvestmentPricesWithHistory,
  fetchInvestmentStrategies
} from '@/api/investment';
import { createTransfer } from '@/api/transfer';
import { fetchAccounts } from '@/api/account';
import { fetchCategories } from '@/api/category';
import { fetchBalanceSheet } from '@/api/report';
import { normalizeCashKind } from '@/types/account';
import type { Account } from '@/types/account';
import type { Category } from '@/types/category';
import type { InvestmentLot, InvestmentStrategy } from '@/types/investment';

const lots = ref<InvestmentLot[]>([]);
const accounts = ref<Account[]>([]);
const categories = ref<Category[]>([]);
const loading = ref(false);
const saving = ref(false);
const statusFilter = ref<'all' | 'open' | 'closed'>('open');
const keyword = ref('');
const buyDateRange = ref<[string | null, string | null]>([null, null]);
const tagFilter = ref<string>('');
const cashAccountFilter = ref<number | null>(null);
const selectedLots = ref<InvestmentLot[]>([]);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const cashBalance = ref<number | null>(null);
const cashBalanceLoading = ref(false);
const cashBalanceAsOf = ref(formatDate(new Date()));
const overallSummary = ref({
  totalCost: 0,
  totalMarket: 0,
  profit: 0,
  hasMarket: false,
  partialMarket: false
});
const dialogVisible = ref(false);
const buyDialogVisible = ref(false);
const buySaving = ref(false);
const buyMode = ref<'create' | 'edit'>('create');
const editingLotId = ref<number | null>(null);
const transferDialogVisible = ref(false);
const transferSaving = ref(false);
const deletingLotId = ref<number | null>(null);
const priceRefreshing = ref(false);
const strategyDialogVisible = ref(false);
const strategyLoading = ref(false);
const strategies = ref<InvestmentStrategy[]>([]);
const hoveredIndicatorRow = ref<InvestmentLot | null>(null);
const hoveredCellRef = ref<HTMLElement | null>(null);
const indicatorTooltipVisible = ref(false);

const buyDateShortcuts = [
  {
    text: '最近一周',
    value: () => {
      const end = new Date();
      const start = new Date();
      start.setDate(end.getDate() - 6);
      return [start, end];
    }
  },
  {
    text: '最近一月',
    value: () => {
      const end = new Date();
      const start = new Date();
      start.setMonth(end.getMonth() - 1);
      return [start, end];
    }
  },
  {
    text: '最近三月',
    value: () => {
      const end = new Date();
      const start = new Date();
      start.setMonth(end.getMonth() - 3);
      return [start, end];
    }
  },
  {
    text: '最近半年',
    value: () => {
      const end = new Date();
      const start = new Date();
      start.setMonth(end.getMonth() - 6);
      return [start, end];
    }
  },
  {
    text: '最近一年',
    value: () => {
      const end = new Date();
      const start = new Date();
      start.setFullYear(end.getFullYear() - 1);
      return [start, end];
    }
  }
];

const saleForm = reactive({
  occurredOn: formatDate(new Date()),
  price: 0,
  targetProfitPct: 5,
  cashAccountId: 0,
  investmentAccountId: 0,
  fee: 0,
  feeCategoryId: null as number | null,
  tax: 0,
  taxCategoryId: null as number | null,
  description: ''
});

const buyForm = reactive({
  occurredOn: formatDate(new Date()),
  securityTicker: '',
  securityName: '',
  quantity: 0,
  price: 0,
  tag: '',
  cashAccountId: 0,
  investmentAccountId: 0,
  fee: 0,
  feeCategoryId: null as number | null,
  tax: 0,
  taxCategoryId: null as number | null,
  description: ''
});

const transferForm = reactive({
  occurredOn: formatDate(new Date()),
  direction: 'bank_to_broker' as 'bank_to_broker' | 'broker_to_bank',
  fromAccountId: 0,
  toAccountId: 0,
  amount: 0,
  description: ''
});

type AllocationRow = {
  lotId: number;
  securityTicker: string;
  securityName: string;
  occurredOn: string;
  price: number;
  remainingQuantity: number;
  quantity: number;
};

const allocationRows = ref<AllocationRow[]>([]);

const cashAccounts = computed(() => accounts.value.filter((item) => item.isActive && item.type === 'cash'));
const bankCashAccounts = computed(() =>
  cashAccounts.value.filter((item) => normalizeCashKind(item.cashKind) === 'bank')
);
const brokerCashAccounts = computed(() =>
  cashAccounts.value.filter((item) => normalizeCashKind(item.cashKind) === 'broker')
);
const investmentAccounts = computed(() =>
  accounts.value.filter((item) => item.isActive && item.type === 'investment')
);
const expenseCategories = computed(() => categories.value.filter((item) => item.kind === 'expense'));
const accountMetaMap = computed(() => new Map(accounts.value.map((item) => [item.id, item])));

const transferFromAccounts = computed(() =>
  transferForm.direction === 'bank_to_broker' ? bankCashAccounts.value : brokerCashAccounts.value
);
const transferToAccounts = computed(() =>
  transferForm.direction === 'bank_to_broker' ? brokerCashAccounts.value : bankCashAccounts.value
);

const filteredLots = computed(() => lots.value);

const summary = computed(() => {
  let totalQuantity = 0;
  let totalCost = 0;
  let totalCostPriced = 0;
  let totalFee = 0;
  let totalMarket = 0;
  let pricedCount = 0;

  for (const lot of lots.value) {
    const qty = lot.quantity || 0;
    totalQuantity += qty;
    const cost = (lot.price || 0) * qty;
    totalCost += cost;
    totalFee += lot.fee || 0;
    if (lot.currentPrice > 0) {
      pricedCount += 1;
      totalCostPriced += cost;
      totalMarket += lot.currentPrice * qty;
    }
  }

  const hasMarket = pricedCount > 0;
  const profit = hasMarket ? totalMarket - totalCostPriced : 0;
  const profitPct = hasMarket && totalCostPriced > 0 ? profit / totalCostPriced : 0;

  return {
    totalQuantity,
    totalCost,
    totalCostPriced,
    totalFee,
    totalMarket,
    profit,
    profitPct,
    hasMarket,
    partialMarket: hasMarket && pricedCount !== lots.value.length
  };
});

const totalAssetCost = computed(() => {
  if (cashBalance.value === null) return null;
  return cashBalance.value + overallSummary.value.totalCost;
});

const totalAssetMarket = computed(() => {
  if (cashBalance.value === null || !overallSummary.value.hasMarket) return null;
  return cashBalance.value + overallSummary.value.totalMarket;
});

const summaryMethod = ({ columns }: { columns: Array<{ label?: string }> }) => {
  const sums: string[] = Array(columns.length).fill('');
  columns.forEach((column, index) => {
    switch (column.label) {
      case '状态':
        sums[index] = '汇总';
        break;
      case '成本价':
        sums[index] = formatNumber(summary.value.totalCost / Math.max(summary.value.totalQuantity, 1), 3);
        break;
      case '手续费':
        sums[index] = formatNumber(summary.value.totalFee, 3);
        break;
      case '总数量':
        sums[index] = formatNumber(summary.value.totalQuantity, 0);
        break;
      case '盈利金额':
        sums[index] = summary.value.hasMarket ? formatNumber(summary.value.profit, 2) : '-';
        break;
      case '盈利点':
        sums[index] = summary.value.hasMarket ? formatPercent(summary.value.profitPct) : '-';
        break;
      case '操作':
        sums[index] = `总成本 ${formatNumber(summary.value.totalCost, 2)}`;
        break;
      default:
        break;
    }
  });
  return sums;
};

const totalQuantity = computed(() => allocationRows.value.reduce((sum, row) => sum + row.quantity, 0));
const totalCost = computed(() => allocationRows.value.reduce((sum, row) => sum + row.quantity * row.price, 0));
const avgCost = computed(() => (totalQuantity.value > 0 ? totalCost.value / totalQuantity.value : 0));
const targetPrice = computed(() => avgCost.value * (1 + saleForm.targetProfitPct / 100));
const grossAmount = computed(() => totalQuantity.value * saleForm.price);
const totalFeeAndTax = computed(() => saleForm.fee + saleForm.tax);
const profitPreview = computed(() => grossAmount.value - totalCost.value - totalFeeAndTax.value);

const buyGrossAmount = computed(() => buyForm.quantity * buyForm.price);
const buyFeeAndTax = computed(() => buyForm.fee + buyForm.tax);
const buyCostAmount = computed(() => buyGrossAmount.value + buyFeeAndTax.value);
const buyCostPrice = computed(() => (buyForm.quantity > 0 ? buyCostAmount.value / buyForm.quantity : 0));

const selectedSecurityLabel = computed(() => {
  if (allocationRows.value.length === 0) return '-';
  const row = allocationRows.value[0];
  return `${row.securityTicker} · ${row.securityName}`;
});

const loadLots = async () => {
  loading.value = true;
  try {
    const params: Record<string, unknown> =
      statusFilter.value === 'all' ? {} : { status: statusFilter.value };
    if (tagFilter.value) {
      params.tag = tagFilter.value;
    }
    if (keyword.value.trim()) {
      params.keyword = keyword.value.trim();
    }
    if (buyDateRange.value?.[0]) {
      params.buy_date_from = buyDateRange.value[0];
    }
    if (buyDateRange.value?.[1]) {
      params.buy_date_to = buyDateRange.value[1];
    }
    if (cashAccountFilter.value) {
      params.cash_account_id = cashAccountFilter.value;
    }
    params.page = page.value;
    params.page_size = pageSize.value;
    const resp = await fetchInvestmentLots(params);
    lots.value = resp.data;
    total.value = resp.total;
    if (resp.summary) {
      overallSummary.value = {
        totalCost: resp.summary.total_cost ?? 0,
        totalMarket: resp.summary.total_market ?? 0,
        profit: resp.summary.profit ?? 0,
        hasMarket: !!resp.summary.has_market,
        partialMarket: !!resp.summary.partial_market
      };
    }
    selectedLots.value = [];
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
};

const loadCashBalance = async () => {
  cashBalanceLoading.value = true;
  try {
    const report = await fetchBalanceSheet({ as_of: cashBalanceAsOf.value });
    const flatAccounts = report.groups.flatMap((group) => group.accounts ?? []).filter(Boolean);
    const balanceMap = new Map(flatAccounts.map((item) => [item.id, item.balance]));

    if (cashAccountFilter.value) {
      cashBalance.value = balanceMap.get(cashAccountFilter.value) ?? 0;
      return;
    }

    let sum = 0;
    for (const item of flatAccounts) {
      const meta = accountMetaMap.value.get(item.id);
      if (!meta) continue;
      if (meta.type === 'cash' && normalizeCashKind(meta.cashKind) === 'broker') {
        sum += item.balance;
      }
    }
    cashBalance.value = sum;
  } catch (error) {
    cashBalance.value = null;
    ElMessage.error((error as Error).message);
  } finally {
    cashBalanceLoading.value = false;
  }
};

const loadMeta = async () => {
  try {
    const [accountList, categoryList] = await Promise.all([fetchAccounts(), fetchCategories()]);
    accounts.value = (accountList ?? []).filter((item): item is Account => Boolean(item));
    categories.value = (categoryList ?? []).filter((item): item is Category => Boolean(item));

    if (!saleForm.cashAccountId && brokerCashAccounts.value.length > 0) {
      saleForm.cashAccountId = brokerCashAccounts.value[0].id;
    }
    if (!saleForm.investmentAccountId && investmentAccounts.value.length > 0) {
      saleForm.investmentAccountId = investmentAccounts.value[0].id;
    }

    if (!buyForm.cashAccountId && brokerCashAccounts.value.length > 0) {
      buyForm.cashAccountId = brokerCashAccounts.value[0].id;
    }
    if (!buyForm.investmentAccountId && investmentAccounts.value.length > 0) {
      buyForm.investmentAccountId = investmentAccounts.value[0].id;
    }

    if (!transferForm.fromAccountId && transferFromAccounts.value.length > 0) {
      transferForm.fromAccountId = transferFromAccounts.value[0].id;
    }
    if (!transferForm.toAccountId && transferToAccounts.value.length > 0) {
      transferForm.toAccountId = transferToAccounts.value[0].id;
    }

    await loadCashBalance();
  } catch (error) {
    ElMessage.error((error as Error).message);
  }
};

const refreshPrices = async () => {
  priceRefreshing.value = true;
  try {
    const resp = await refreshInvestmentPricesWithHistory(60);
    const detail = `已更新 ${resp.updated} 条，跳过 ${resp.skipped} 条`;
    ElMessage.success(`现价刷新完成，${detail}`);
    await loadLots();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    priceRefreshing.value = false;
  }
};

const loadStrategies = async () => {
  strategyLoading.value = true;
  try {
    strategies.value = await fetchInvestmentStrategies();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    strategyLoading.value = false;
  }
};

async function openStrategyDialog() {
  strategyDialogVisible.value = true;
  await loadStrategies();
}

const onSelectionChange = (rows: InvestmentLot[]) => {
  selectedLots.value = rows;
};

const onCellMouseEnter = (row: InvestmentLot, _column: unknown, cell: HTMLElement | undefined) => {
  hoveredIndicatorRow.value = row;
  hoveredCellRef.value = cell ?? null;
  indicatorTooltipVisible.value = true;
};

const onCellMouseLeave = () => {
  indicatorTooltipVisible.value = false;
};

const isSelectable = (row: InvestmentLot) => row.status === 'open' && row.remainingQuantity > 0;

const openBuyDialog = () => {
  buyMode.value = 'create';
  editingLotId.value = null;
  buyForm.occurredOn = formatDate(new Date());
  buyForm.securityTicker = '';
  buyForm.securityName = '';
  buyForm.tag = '';
  buyForm.quantity = 0;
  buyForm.price = 0;
  buyForm.fee = 0;
  buyForm.tax = 0;
  buyForm.feeCategoryId = null;
  buyForm.taxCategoryId = null;
  buyForm.description = '';
  buyDialogVisible.value = true;
};

const openTransferDialog = () => {
  transferForm.occurredOn = formatDate(new Date());
  transferForm.amount = 0;
  transferForm.description = '';
  if (!transferForm.fromAccountId && transferFromAccounts.value.length > 0) {
    transferForm.fromAccountId = transferFromAccounts.value[0].id;
  }
  if (!transferForm.toAccountId && transferToAccounts.value.length > 0) {
    transferForm.toAccountId = transferToAccounts.value[0].id;
  }
  transferDialogVisible.value = true;
};

watch(
  () => transferForm.direction,
  () => {
    if (!transferFromAccounts.value.some((item) => item.id === transferForm.fromAccountId)) {
      transferForm.fromAccountId = transferFromAccounts.value[0]?.id || 0;
    }
    if (!transferToAccounts.value.some((item) => item.id === transferForm.toAccountId)) {
      transferForm.toAccountId = transferToAccounts.value[0]?.id || 0;
    }
  }
);

const openBuyDialogFromRow = (row: InvestmentLot) => {
  buyMode.value = 'create';
  editingLotId.value = null;
  buyForm.occurredOn = row.occurredOn || formatDate(new Date());
  buyForm.securityTicker = row.securityTicker;
  buyForm.securityName = row.securityName;
  buyForm.tag = row.tag || '';
  buyForm.quantity = row.quantity;
  buyForm.price = row.tradePrice > 0 ? row.tradePrice : row.price;
  buyForm.fee = row.fee || 0;
  buyForm.tax = row.tax || 0;
  buyForm.feeCategoryId = null;
  buyForm.taxCategoryId = null;
  buyForm.description = '';
  buyDialogVisible.value = true;
};

const openEditBuyDialog = (row: InvestmentLot) => {
  buyMode.value = 'edit';
  editingLotId.value = row.lotId;
  buyForm.occurredOn = row.occurredOn || formatDate(new Date());
  buyForm.securityTicker = row.securityTicker;
  buyForm.securityName = row.securityName;
  buyForm.tag = row.tag || '';
  buyForm.quantity = row.quantity;
  buyForm.price = row.tradePrice > 0 ? row.tradePrice : row.price;
  buyForm.fee = row.fee || 0;
  buyForm.tax = row.tax || 0;
  buyForm.feeCategoryId = null;
  buyForm.taxCategoryId = null;
  buyForm.description = '';
  buyDialogVisible.value = true;
};

const confirmDeleteLot = (row: InvestmentLot) => {
  if (row.allocatedQuantity > 0) {
    ElMessage.warning('该批次已匹配卖单，无法删除');
    return;
  }
  ElMessageBox.confirm(`确认删除买入批次「${row.securityTicker} ${row.occurredOn}」？`, '提示', { type: 'warning' })
    .then(() =>
      ElMessageBox.confirm('删除后将把金额退回到资金账户，是否继续？', '二次确认', { type: 'warning' })
    )
    .then(async () => {
      deletingLotId.value = row.lotId;
      try {
        await deleteInvestmentBuy(row.lotId);
        await loadLots();
        ElMessage.success('已删除并退回资金');
      } catch (error) {
        ElMessage.error((error as Error).message);
      } finally {
        deletingLotId.value = null;
      }
    })
    .catch(() => undefined);
};

const openSellDialog = () => {
  if (selectedLots.value.length === 0) {
    ElMessage.warning('请先选择未匹配批次');
    return;
  }

  const securityId = selectedLots.value[0].securityId;
  if (selectedLots.value.some((row) => row.securityId !== securityId)) {
    ElMessage.warning('一次只能选择同一标的的批次');
    return;
  }

  allocationRows.value = selectedLots.value
    .filter(isSelectable)
    .map((row) => ({
      lotId: row.lotId,
      securityTicker: row.securityTicker,
      securityName: row.securityName,
      occurredOn: row.occurredOn,
      price: row.price,
      remainingQuantity: Math.max(row.remainingQuantity, 0),
      quantity: Math.max(row.remainingQuantity, 0)
    }));

  if (allocationRows.value.length === 0) {
    ElMessage.warning('所选批次均已匹配');
    return;
  }

  saleForm.occurredOn = formatDate(new Date());
  saleForm.price = 0;
  saleForm.fee = 0;
  saleForm.tax = 0;
  saleForm.description = '';

  dialogVisible.value = true;
};

const submitBuy = async () => {
  if (!buyForm.occurredOn) {
    ElMessage.error('请选择买入日期');
    return;
  }
  if (!buyForm.securityTicker.trim() || !buyForm.securityName.trim()) {
    ElMessage.error('请输入标的代码与名称');
    return;
  }
  if (buyForm.quantity <= 0 || buyForm.price <= 0) {
    ElMessage.error('请输入买入数量与成交价');
    return;
  }
  if (!buyForm.cashAccountId) {
    ElMessage.error('请选择资金账户');
    return;
  }
  if (!buyForm.investmentAccountId) {
    ElMessage.error('请选择持仓账户');
    return;
  }

  const payload = {
    occurred_on: buyForm.occurredOn,
    security_ticker: buyForm.securityTicker.trim(),
    security_name: buyForm.securityName.trim(),
    tag: buyForm.tag?.trim() || undefined,
    cash_account_id: buyForm.cashAccountId,
    investment_account_id: buyForm.investmentAccountId,
    quantity: buyForm.quantity,
    price: buyForm.price,
    fee: buyForm.fee || 0,
    fee_category_id: buyForm.feeCategoryId ?? undefined,
    tax: buyForm.tax || 0,
    tax_category_id: buyForm.taxCategoryId ?? undefined,
    description: buyForm.description?.trim() || undefined
  };

  buySaving.value = true;
  try {
    if (buyMode.value === 'edit' && editingLotId.value) {
      await updateInvestmentBuy(editingLotId.value, payload);
      ElMessage.success('买入记录已更新');
    } else {
      await createInvestmentBuy(payload);
      ElMessage.success('买入记录已创建');
    }
    buyDialogVisible.value = false;
    await loadLots();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    buySaving.value = false;
  }
};

const submitTransfer = async () => {
  if (!transferForm.occurredOn) {
    ElMessage.error('请选择转账日期');
    return;
  }
  if (!transferForm.fromAccountId || !transferForm.toAccountId) {
    ElMessage.error('请选择转出与转入账户');
    return;
  }
  if (transferForm.fromAccountId === transferForm.toAccountId) {
    ElMessage.error('转出与转入账户不能相同');
    return;
  }
  if (transferForm.amount <= 0) {
    ElMessage.error('请输入转账金额');
    return;
  }

  transferSaving.value = true;
  try {
    await createTransfer({
      occurred_on: transferForm.occurredOn,
      from_account_id: transferForm.fromAccountId,
      to_account_id: transferForm.toAccountId,
      amount: transferForm.amount,
      description: transferForm.description?.trim() || undefined
    });
    ElMessage.success('转账已完成');
    transferDialogVisible.value = false;
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    transferSaving.value = false;
  }
};

const submitSale = async () => {
  if (!saleForm.occurredOn) {
    ElMessage.error('请选择卖出日期');
    return;
  }
  if (saleForm.price <= 0) {
    ElMessage.error('请输入卖出价格');
    return;
  }
  if (!saleForm.cashAccountId) {
    ElMessage.error('请选择资金账户');
    return;
  }
  if (!saleForm.investmentAccountId) {
    ElMessage.error('请选择持仓账户');
    return;
  }

  const allocations = allocationRows.value
    .filter((row) => row.quantity > 0)
    .map((row) => ({ buy_lot_id: row.lotId, quantity: row.quantity }));

  if (allocations.length === 0) {
    ElMessage.error('请填写卖出数量');
    return;
  }

  const securityId = selectedLots.value[0]?.securityId ?? 0;
  if (!securityId) {
    ElMessage.error('标的信息缺失');
    return;
  }

  saving.value = true;
  try {
    await createInvestmentSale({
      occurred_on: saleForm.occurredOn,
      security_id: securityId,
      cash_account_id: saleForm.cashAccountId,
      investment_account_id: saleForm.investmentAccountId,
      price: saleForm.price,
      fee: saleForm.fee || 0,
      fee_category_id: saleForm.feeCategoryId ?? undefined,
      tax: saleForm.tax || 0,
      tax_category_id: saleForm.taxCategoryId ?? undefined,
      description: saleForm.description?.trim() || undefined,
      allocations
    });

    ElMessage.success('卖出记录已创建');
    dialogVisible.value = false;
    selectedLots.value = [];
    await loadLots();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
};

function reloadLots() {
  page.value = 1;
  loadLots();
}

watch(statusFilter, () => {
  reloadLots();
});

watch(tagFilter, () => {
  reloadLots();
});

watch(keyword, () => {
  reloadLots();
});

watch(
  buyDateRange,
  () => {
    reloadLots();
  },
  { deep: true }
);

watch(cashAccountFilter, () => {
  reloadLots();
  loadCashBalance();
});

const onPageChange = (next: number) => {
  page.value = next;
  loadLots();
};

const onPageSizeChange = (next: number) => {
  pageSize.value = next;
  page.value = 1;
  loadLots();
};

onMounted(() => {
  loadMeta();
  loadLots();
});

function normalizeStrategyParams(params: unknown): Record<string, any> {
  if (!params) return {};
  if (typeof params === 'string') {
    try {
      return JSON.parse(params) as Record<string, any>;
    } catch {
      return {};
    }
  }
  if (typeof params === 'object') {
    return params as Record<string, any>;
  }
  return {};
}

function readStrategyField(params: Record<string, any>, path: string[]): unknown {
  let current: any = params;
  for (const key of path) {
    if (!current || typeof current !== 'object') return null;
    current = current[key];
  }
  return current ?? null;
}

function strategyEntrySummary(row: InvestmentStrategy): string {
  const params = normalizeStrategyParams(row.params);
  const trigger = readStrategyField(params, ['entry', 'trigger']);
  const timing = readStrategyField(params, ['entry', 'timing']);
  const position = readStrategyField(params, ['entry', 'position_size_pct']);
  const parts: string[] = [];
  if (typeof trigger === 'string' && trigger) parts.push(trigger);
  if (typeof timing === 'string' && timing) parts.push(timing);
  if (typeof position === 'number') parts.push(`仓位 ${(position * 100).toFixed(0)}%`);
  return parts.length > 0 ? parts.join(' / ') : '-';
}

function strategyExitSummary(row: InvestmentStrategy): string {
  const params = normalizeStrategyParams(row.params);
  const stopLoss = readStrategyField(params, ['exit', 'stop_loss_pct']);
  const takeProfit = readStrategyField(params, ['exit', 'take_profit_pct']);
  const takeProfitAlt = readStrategyField(params, ['exit', 'take_profit_pct_alt']);
  const timeStop = readStrategyField(params, ['exit', 'time_stop_days']);
  const parts: string[] = [];
  if (typeof stopLoss === 'number') parts.push(`止损 ${(stopLoss * 100).toFixed(1)}%`);
  if (typeof takeProfit === 'number') {
    const tpAlt =
      typeof takeProfitAlt === 'number' ? `或 ${(takeProfitAlt * 100).toFixed(1)}%` : '';
    parts.push(`止盈 ${(takeProfit * 100).toFixed(1)}%${tpAlt}`);
  }
  if (typeof timeStop === 'number') parts.push(`最长 ${timeStop} 天`);
  return parts.length > 0 ? parts.join(' / ') : '-';
}

function strategyProfileSummary(row: InvestmentStrategy): string {
  const params = normalizeStrategyParams(row.params);
  const market = readStrategyField(params, ['profile', 'market']);
  const timeframe = readStrategyField(params, ['profile', 'timeframe']);
  const holding = readStrategyField(params, ['profile', 'holding_period_days']);
  const parts: string[] = [];
  if (typeof market === 'string' && market) parts.push(market);
  if (typeof timeframe === 'string' && timeframe) parts.push(timeframe);
  if (typeof holding === 'number') parts.push(`${holding}天`);
  return parts.length > 0 ? parts.join(' / ') : '-';
}

function formatStrategyParams(row: InvestmentStrategy): string {
  const params = normalizeStrategyParams(row.params);
  try {
    return JSON.stringify(params, null, 2);
  } catch {
    return String(row.params ?? '');
  }
}

function formatNumber(value: number, digits = 2): string {
  if (!Number.isFinite(value)) return '-';
  return value.toFixed(digits);
}

function formatIndicator(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-';
  return value.toFixed(3);
}

function rowClassName({ row }: { row: InvestmentLot }): string {
  return row.tag === '打野' ? 'row-tag-daye' : '';
}

function profitAmount(row: InvestmentLot): number | null {
  if (!row || row.currentPrice <= 0) return null;
  return (row.currentPrice - row.price) * row.quantity;
}

function profitPct(row: InvestmentLot): number | null {
  if (!row || row.currentPrice <= 0 || row.price <= 0) return null;
  return (row.currentPrice - row.price) / row.price;
}

function profitClass(value: number): string {
  if (!Number.isFinite(value)) return 'profit-neutral';
  if (value > 0) return 'profit-positive';
  if (value < 0) return 'profit-negative';
  return 'profit-neutral';
}

function formatDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function formatPercent(value: number, digits = 2): string {
  if (!Number.isFinite(value)) return '-';
  return `${(value * 100).toFixed(digits)}%`;
}
</script>

<style scoped>
.security-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.security-ticker {
  font-weight: 600;
}

.security-name {
  font-size: 12px;
  color: var(--app-text-secondary);
}

.pagination-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.overall-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: center;
  margin-top: 12px;
  padding: 10px 14px;
  background: var(--app-surface-muted);
  border-radius: 10px;
  border: 1px solid var(--app-border-soft);
}

.overall-item {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.overall-label {
  font-size: 12px;
  color: var(--app-text-secondary);
}

.overall-value {
  font-size: 16px;
  font-weight: 600;
  color: var(--app-text-primary);
}

.overall-note {
  font-size: 12px;
  color: var(--app-text-muted);
}

.profit-cell {
  font-weight: 600;
}

.profit-positive {
  color: var(--app-negative);
}

.profit-negative {
  color: var(--app-positive);
}

.profit-neutral {
  color: var(--app-text-secondary);
}

.indicator-tooltip {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  line-height: 1.3;
}

.strategy-name {
  font-weight: 600;
}

.strategy-kind {
  font-size: 12px;
  color: var(--app-text-secondary);
}

.strategy-summary {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--app-text-primary);
}

.strategy-json {
  margin: 0;
  font-size: 12px;
  line-height: 1.4;
  white-space: pre-wrap;
  word-break: break-word;
}

:deep(.el-table__body tr.row-tag-daye > td) {
  background: var(--app-row-daye-bg) !important;
}

.toolbar-investments {
  flex-direction: row;
  align-items: center;
  flex-wrap: wrap;
}

.toolbar-filters {
  display: flex;
  gap: 10px;
  flex: 1;
  flex-wrap: nowrap;
  min-width: 520px;
}

.toolbar-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.dialog-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
  padding: 12px 16px;
  border-radius: 12px;
  background: var(--app-surface-muted);
  border: 1px solid var(--app-border-soft);
}

.summary-label {
  font-size: 12px;
  color: var(--app-text-secondary);
}

.summary-value {
  font-size: 16px;
  font-weight: 600;
  color: var(--app-text-primary);
}

.inline-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.hint-text {
  font-size: 12px;
  color: var(--app-text-secondary);
}

.summary-inline {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 12px;
}

.profit-positive {
  color: var(--app-negative);
  font-weight: 600;
}

.profit-negative {
  color: var(--app-positive);
  font-weight: 600;
}

@media (max-width: 900px) {
  .dialog-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>

<style scoped>
.table-scroll {
  width: 100%;
  overflow-x: auto;
}
</style>
