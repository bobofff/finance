<template>
  <div class="section-header">
    <div>
      <h1>投资批次</h1>
      <div class="light-text">选择买入批次，计算均价并快速生成卖出记录</div>
    </div>
    <div class="toolbar">
      <el-select v-model="statusFilter" placeholder="状态" style="min-width: 140px">
        <el-option label="全部" value="all" />
        <el-option label="未匹配" value="open" />
        <el-option label="已匹配" value="closed" />
      </el-select>
      <el-input v-model="keyword" placeholder="搜索标的" clearable style="min-width: 200px" />
      <el-button type="primary" plain @click="openBuyDialog">新增买入</el-button>
      <el-button type="primary" plain @click="openTransferDialog">银证转账</el-button>
      <el-button type="primary" :disabled="selectedLots.length === 0" @click="openSellDialog">批次卖出</el-button>
      <el-button :icon="RefreshRight" :loading="loading" @click="loadLots">刷新</el-button>
    </div>
  </div>

  <div class="card">
    <el-table
      :data="filteredLots"
      stripe
      border
      v-loading="loading"
      style="width: 100%"
      row-key="lotId"
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
      <el-table-column label="标的" min-width="200">
        <template #default="{ row }">
          <div class="security-cell">
            <div class="security-ticker">{{ row.securityTicker }}</div>
            <div class="security-name">{{ row.securityName }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="occurredOn" label="买入日期" width="130" />
      <el-table-column label="成交价" width="120" align="right">
        <template #default="{ row }">
          {{ formatNumber(row.tradePrice > 0 ? row.tradePrice : row.price, 4) }}
        </template>
      </el-table-column>
      <el-table-column label="成本价" width="120" align="right">
        <template #default="{ row }">{{ formatNumber(row.price, 4) }}</template>
      </el-table-column>
      <el-table-column label="手续费" width="110" align="right">
        <template #default="{ row }">{{ formatNumber(row.fee, 4) }}</template>
      </el-table-column>
      <el-table-column label="税费" width="110" align="right">
        <template #default="{ row }">{{ formatNumber(row.tax, 4) }}</template>
      </el-table-column>
      <el-table-column label="总数量" width="120" align="right">
        <template #default="{ row }">{{ formatNumber(row.quantity, 4) }}</template>
      </el-table-column>
      <el-table-column label="已匹配" width="120" align="right">
        <template #default="{ row }">{{ formatNumber(row.allocatedQuantity, 4) }}</template>
      </el-table-column>
      <el-table-column label="可卖数量" width="120" align="right">
        <template #default="{ row }">{{ formatNumber(Math.max(row.remainingQuantity, 0), 4) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" align="right">
        <template #default="{ row }">
          <div class="table-actions">
            <el-button size="small" @click="openBuyDialogFromRow(row)">快速添加</el-button>
            <el-button size="small" type="primary" plain :disabled="row.allocatedQuantity > 0" @click="openEditBuyDialog(row)">
              修改
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>
  </div>

  <el-dialog v-model="dialogVisible" title="批次卖出" width="960px" destroy-on-close>
    <div class="dialog-summary">
      <div>
        <div class="summary-label">标的</div>
        <div class="summary-value">{{ selectedSecurityLabel }}</div>
      </div>
      <div>
        <div class="summary-label">总数量</div>
        <div class="summary-value">{{ formatNumber(totalQuantity, 4) }}</div>
      </div>
      <div>
        <div class="summary-label">平均成本</div>
        <div class="summary-value">{{ formatNumber(avgCost, 4) }}</div>
      </div>
      <div>
        <div class="summary-label">目标价</div>
        <div class="summary-value">{{ formatNumber(targetPrice, 4) }}</div>
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
        <template #default="{ row }">{{ formatNumber(row.price, 4) }}</template>
      </el-table-column>
      <el-table-column label="可卖数量" width="140" align="right">
        <template #default="{ row }">{{ formatNumber(row.remainingQuantity, 4) }}</template>
      </el-table-column>
      <el-table-column label="卖出数量" width="180">
        <template #default="{ row }">
          <el-input-number
            v-model="row.quantity"
            :min="0"
            :max="row.remainingQuantity"
            :step="0.0001"
            :precision="4"
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
        <el-input-number v-model="saleForm.price" :min="0" :step="0.01" :precision="4" controls-position="right" />
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
          <span class="hint-text">目标价 {{ formatNumber(targetPrice, 4) }}</span>
        </div>
      </el-form-item>
      <el-form-item label="资金账户">
        <el-select v-model="saleForm.cashAccountId" placeholder="选择证券资金账户" style="width: 100%">
          <el-option
            v-for="account in cashAccounts"
            :key="account.id"
            :label="account.name"
            :value="account.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="持仓账户">
        <el-select v-model="saleForm.investmentAccountId" placeholder="选择持仓账户" style="width: 100%">
          <el-option
            v-for="account in investmentAccounts"
            :key="account.id"
            :label="account.name"
            :value="account.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="手续费">
        <div class="inline-row">
          <el-input-number v-model="saleForm.fee" :min="0" :step="0.01" :precision="4" controls-position="right" />
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
      <el-form-item label="税费">
        <div class="inline-row">
          <el-input-number v-model="saleForm.tax" :min="0" :step="0.01" :precision="4" controls-position="right" />
          <el-select v-model="saleForm.taxCategoryId" clearable placeholder="税费分类" style="min-width: 180px">
            <el-option
              v-for="category in expenseCategories"
              :key="category.id"
              :label="category.name"
              :value="category.id"
            />
          </el-select>
        </div>
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="saleForm.description" placeholder="可选：卖出说明" />
      </el-form-item>
      <el-form-item label="盈利预估">
        <div class="summary-inline">
          <span>毛收入 {{ formatNumber(grossAmount, 4) }}</span>
          <span>成本 {{ formatNumber(totalCost, 4) }}</span>
          <span>费用 {{ formatNumber(totalFeeAndTax, 4) }}</span>
          <span :class="profitPreview >= 0 ? 'profit-positive' : 'profit-negative'">
            预计盈亏 {{ formatNumber(profitPreview, 4) }}
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
        <div class="summary-value">{{ formatNumber(buyGrossAmount, 4) }}</div>
      </div>
      <div>
        <div class="summary-label">含费成本</div>
        <div class="summary-value">{{ formatNumber(buyCostAmount, 4) }}</div>
      </div>
      <div>
        <div class="summary-label">成本价</div>
        <div class="summary-value">{{ formatNumber(buyCostPrice, 4) }}</div>
      </div>
      <div>
        <div class="summary-label">手续费+税费</div>
        <div class="summary-value">{{ formatNumber(buyFeeAndTax, 4) }}</div>
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
      <el-form-item label="买入数量">
        <el-input-number v-model="buyForm.quantity" :min="0" :step="0.0001" :precision="4" controls-position="right" />
      </el-form-item>
      <el-form-item label="成交价">
        <el-input-number v-model="buyForm.price" :min="0" :step="0.01" :precision="4" controls-position="right" />
      </el-form-item>
      <el-form-item label="资金账户">
        <el-select v-model="buyForm.cashAccountId" placeholder="选择证券资金账户" style="width: 100%">
          <el-option
            v-for="account in cashAccounts"
            :key="account.id"
            :label="account.name"
            :value="account.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="持仓账户">
        <el-select v-model="buyForm.investmentAccountId" placeholder="选择持仓账户" style="width: 100%">
          <el-option
            v-for="account in investmentAccounts"
            :key="account.id"
            :label="account.name"
            :value="account.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="手续费">
        <div class="inline-row">
          <el-input-number v-model="buyForm.fee" :min="0" :step="0.01" :precision="4" controls-position="right" />
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
      <el-form-item label="税费">
        <div class="inline-row">
          <el-input-number v-model="buyForm.tax" :min="0" :step="0.01" :precision="4" controls-position="right" />
          <el-select v-model="buyForm.taxCategoryId" clearable placeholder="税费分类" style="min-width: 180px">
            <el-option
              v-for="category in expenseCategories"
              :key="category.id"
              :label="category.name"
              :value="category.id"
            />
          </el-select>
        </div>
      </el-form-item>
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
      <el-form-item label="转账日期">
        <el-date-picker v-model="transferForm.occurredOn" type="date" value-format="YYYY-MM-DD" />
      </el-form-item>
      <el-form-item label="转出账户">
        <el-select v-model="transferForm.fromAccountId" placeholder="选择银行或资金账户" style="width: 100%">
          <el-option
            v-for="account in cashAccounts"
            :key="account.id"
            :label="account.name"
            :value="account.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="转入账户">
        <el-select v-model="transferForm.toAccountId" placeholder="选择证券资金账户" style="width: 100%">
          <el-option
            v-for="account in cashAccounts"
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
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { RefreshRight } from '@element-plus/icons-vue';
import { fetchInvestmentLots, createInvestmentBuy, createInvestmentSale, updateInvestmentBuy } from '@/api/investment';
import { createTransfer } from '@/api/transfer';
import { fetchAccounts } from '@/api/account';
import { fetchCategories } from '@/api/category';
import type { Account } from '@/types/account';
import type { Category } from '@/types/category';
import type { InvestmentLot } from '@/types/investment';

const lots = ref<InvestmentLot[]>([]);
const accounts = ref<Account[]>([]);
const categories = ref<Category[]>([]);
const loading = ref(false);
const saving = ref(false);
const statusFilter = ref<'all' | 'open' | 'closed'>('open');
const keyword = ref('');
const selectedLots = ref<InvestmentLot[]>([]);
const dialogVisible = ref(false);
const buyDialogVisible = ref(false);
const buySaving = ref(false);
const buyMode = ref<'create' | 'edit'>('create');
const editingLotId = ref<number | null>(null);
const transferDialogVisible = ref(false);
const transferSaving = ref(false);

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
const investmentAccounts = computed(() =>
  accounts.value.filter((item) => item.isActive && item.type === 'investment')
);
const expenseCategories = computed(() => categories.value.filter((item) => item.kind === 'expense'));

const filteredLots = computed(() => {
  const value = keyword.value.trim().toLowerCase();
  if (!value) return lots.value;
  return lots.value.filter((lot) => {
    const ticker = lot.securityTicker.toLowerCase();
    const name = lot.securityName.toLowerCase();
    return ticker.includes(value) || name.includes(value);
  });
});

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
    const params = statusFilter.value === 'all' ? {} : { status: statusFilter.value };
    lots.value = await fetchInvestmentLots(params);
    selectedLots.value = [];
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
};

const loadMeta = async () => {
  try {
    const [accountList, categoryList] = await Promise.all([fetchAccounts(), fetchCategories()]);
    accounts.value = accountList;
    categories.value = categoryList;

    if (!saleForm.cashAccountId && cashAccounts.value.length > 0) {
      saleForm.cashAccountId = cashAccounts.value[0].id;
    }
    if (!saleForm.investmentAccountId && investmentAccounts.value.length > 0) {
      saleForm.investmentAccountId = investmentAccounts.value[0].id;
    }

    if (!buyForm.cashAccountId && cashAccounts.value.length > 0) {
      buyForm.cashAccountId = cashAccounts.value[0].id;
    }
    if (!buyForm.investmentAccountId && investmentAccounts.value.length > 0) {
      buyForm.investmentAccountId = investmentAccounts.value[0].id;
    }

    if (!transferForm.fromAccountId && cashAccounts.value.length > 0) {
      transferForm.fromAccountId = cashAccounts.value[0].id;
    }
    if (!transferForm.toAccountId && cashAccounts.value.length > 1) {
      transferForm.toAccountId = cashAccounts.value[1].id;
    }
  } catch (error) {
    ElMessage.error((error as Error).message);
  }
};

const onSelectionChange = (rows: InvestmentLot[]) => {
  selectedLots.value = rows;
};

const isSelectable = (row: InvestmentLot) => row.status === 'open' && row.remainingQuantity > 0;

const openBuyDialog = () => {
  buyMode.value = 'create';
  editingLotId.value = null;
  buyForm.occurredOn = formatDate(new Date());
  buyForm.securityTicker = '';
  buyForm.securityName = '';
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
  if (!transferForm.fromAccountId && cashAccounts.value.length > 0) {
    transferForm.fromAccountId = cashAccounts.value[0].id;
  }
  if (!transferForm.toAccountId && cashAccounts.value.length > 1) {
    transferForm.toAccountId = cashAccounts.value[1].id;
  }
  transferDialogVisible.value = true;
};

const openBuyDialogFromRow = (row: InvestmentLot) => {
  buyMode.value = 'create';
  editingLotId.value = null;
  buyForm.occurredOn = row.occurredOn || formatDate(new Date());
  buyForm.securityTicker = row.securityTicker;
  buyForm.securityName = row.securityName;
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
  buyForm.quantity = row.quantity;
  buyForm.price = row.tradePrice > 0 ? row.tradePrice : row.price;
  buyForm.fee = row.fee || 0;
  buyForm.tax = row.tax || 0;
  buyForm.feeCategoryId = null;
  buyForm.taxCategoryId = null;
  buyForm.description = '';
  buyDialogVisible.value = true;
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

watch(statusFilter, () => {
  loadLots();
});

onMounted(() => {
  loadMeta();
  loadLots();
});

function formatNumber(value: number, digits = 2): string {
  if (!Number.isFinite(value)) return '-';
  return value.toFixed(digits);
}

function formatDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
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
  color: #6b7280;
}

.dialog-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
  padding: 12px 16px;
  border-radius: 12px;
  background: #f8fafc;
}

.summary-label {
  font-size: 12px;
  color: #6b7280;
}

.summary-value {
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
}

.inline-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.hint-text {
  font-size: 12px;
  color: #6b7280;
}

.summary-inline {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 12px;
}

.profit-positive {
  color: #16a34a;
  font-weight: 600;
}

.profit-negative {
  color: #dc2626;
  font-weight: 600;
}

@media (max-width: 900px) {
  .dialog-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
