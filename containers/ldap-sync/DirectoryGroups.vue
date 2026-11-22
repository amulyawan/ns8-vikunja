<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<template>
	<Card :title="t('team.ns8Directory.title')" class="directory-groups">
		<p>{{ t('team.ns8Directory.help') }}</p>
		<p v-if="statusLoading">{{ t('misc.loading') }}</p>
		<p v-else-if="!enabled">{{ t('team.ns8Directory.enableSync') }}</p>
		<template v-else>
			<form class="permission-check" @submit.prevent="loadGroups">
				<label :for="permissionInputId">{{ t('team.ns8Directory.password') }}</label>
				<input :id="permissionInputId" v-model="permissionPassword" type="password" class="input"
					autocomplete="current-password" :disabled="busy || !!pendingChange" required>
				<XButton class="mt-2" :disabled="busy || !!pendingChange || !permissionPassword" @click="loadGroups">
					{{ t('team.ns8Directory.checkPermissions') }}
				</XButton>
			</form>
			<p v-if="error" role="alert" class="help is-danger">{{ error }}</p>
			<p v-if="message" role="status" class="help">{{ message }}</p>
			<p v-if="groupsLoaded && groups.length === 0">{{ t('team.ns8Directory.noPermission') }}</p>
			<template v-if="groups.length">
				<label :for="groupInputId">{{ t('team.ns8Directory.group') }}</label>
				<select :id="groupInputId" v-model="selectedGroupId" class="input" :disabled="busy || !!pendingChange" @change="clearSelection">
					<option value="" disabled>{{ t('team.ns8Directory.selectGroup') }}</option>
					<option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
				</select>
				<template v-if="selectedGroup">
					<p class="mt-2">{{ selectedGroup.description }}</p>
					<div class="field has-addons mt-3">
						<div class="control is-expanded">
							<Multiselect v-model="newMember" :loading="usersLoading" :placeholder="t('team.edit.search')"
								:search-results="foundUsers" label="username" @search="findUser">
								<template #searchResult="{option: person}"><User :user="person" :avatar-size="24" /></template>
							</Multiselect>
						</div>
						<XButton :disabled="busy || !newMember?.id" @click="beginAdd">{{ t('team.edit.addUser') }}</XButton>
					</div>
					<p class="help">{{ t('team.ns8Directory.membershipHelp') }}</p>
					<div class="has-horizontal-overflow">
						<table class="table is-fullwidth is-striped">
							<thead><tr><th>{{ t('team.ns8Directory.person') }}</th><th>{{ t('team.ns8Directory.membership') }}</th><th /></tr></thead>
							<tbody><tr v-for="member in selectedGroup.members" :key="member.id">
								<td>{{ member.name || member.username }} <small>({{ member.username }})</small></td>
								<td>{{ t(`team.ns8Directory.${member.kind}`) }}</td>
								<td><XButton :disabled="busy || member.kind !== 'direct'"
									@click="beginRemove(member)">{{ t('team.ns8Directory.remove') }}</XButton></td>
							</tr></tbody>
						</table>
					</div>
				</template>
			</template>
		</template>
		<div v-if="pendingChange" class="directory-confirm-backdrop">
			<form role="dialog" aria-modal="true" :aria-labelledby="confirmTitleId" class="directory-confirm"
				@submit.prevent="confirmChange" @keydown.esc.prevent="cancelChange" @keydown.tab="trapConfirmFocus">
				<h3 :id="confirmTitleId">{{ t(pendingChange.desired ? 'team.ns8Directory.confirmAdd' : 'team.ns8Directory.confirmRemove',
					{name:pendingChange.name, group:pendingChange.groupName}) }}</h3>
				<p>{{ t('team.ns8Directory.confirmHelp') }}</p>
				<label :for="confirmInputId">{{ t('team.ns8Directory.password') }}</label>
				<input :id="confirmInputId" ref="confirmInput" v-model="changePassword" type="password"
					class="input" autocomplete="current-password" :disabled="busy" required>
				<div class="mt-3">
					<XButton :disabled="busy || !changePassword" @click="confirmChange">{{ t('team.ns8Directory.confirm') }}</XButton>
					<XButton class="ml-2" :disabled="busy" @click="cancelChange">{{ t('misc.cancel') }}</XButton>
				</div>
			</form>
		</div>
	</Card>
</template>

<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, ref, useId} from 'vue'
import {useI18n} from 'vue-i18n'
import Card from '@/components/misc/Card.vue'
import Multiselect from '@/components/input/Multiselect.vue'
import User from '@/components/misc/User.vue'
import {client} from '@/client/generated/client.gen'
import type {User as ApiUser} from '@/client/generated'
import {queryClient} from '@/client/queryClient'
import {useUserSearch} from '@/composables/useUserSearch'

const props = defineProps<{initialGroupId?: string}>()
const {t} = useI18n({useScope:'global'})
type Member = {id:number; username:string; name:string; kind:'direct'|'primary'|'inherited'; disabled:boolean}
type Group = {id:string; name:string; description:string; members:Member[]}
type ChangeResult = {confirmed:boolean; changed:boolean; refresh_pending:boolean; effective_member:boolean; groups?:Group[]}
const enabled=ref(false), statusLoading=ref(true), busy=ref(false), groupsLoaded=ref(false)
const groups=ref<Group[]>([]), selectedGroupId=ref(''), error=ref(''), message=ref('')
const permissionPassword=ref(''), changePassword=ref('')
const pendingChange=ref<{id:number; name:string; desired:boolean; groupId:string; groupName:string}|null>(null)
const confirmInput=ref<HTMLInputElement|null>(null)
const id=useId(), permissionInputId=`directory-password-${id}`, confirmInputId=`directory-change-password-${id}`
const groupInputId=`directory-group-${id}`, confirmTitleId=`directory-confirm-${id}`
const selectedGroup=computed(()=>groups.value.find(group=>group.id===selectedGroupId.value))
const userSearch=ref(''), newMember=ref<ApiUser|null>(null)
const {users:userResults,isFetching:usersLoading}=useUserSearch(userSearch)
const foundUsers=computed(()=>userResults.value.filter(person=>!selectedGroup.value?.members.some(member=>member.id===person.id)))

function errorText(problem:unknown):string {
	const value=problem as {detail?:string; message?:string}
	return value?.detail || value?.message || t('team.ns8Directory.failed')
}
function clearSelection(){newMember.value=null;userSearch.value='';message.value='';error.value=''}
function findUser(query:string){if(!query)clearSelection();userSearch.value=query}

onMounted(async()=>{
	try{const {data}=await client.get<{200:{enabled:boolean}}>({url:'/ns8/directory/status'});enabled.value=!!data?.enabled}
	catch(problem){error.value=errorText(problem)}finally{statusLoading.value=false}
})
onBeforeUnmount(()=>{permissionPassword.value='';changePassword.value='';pendingChange.value=null})

async function loadGroups(){
	if(busy.value||!permissionPassword.value)return
	busy.value=true;error.value='';message.value=''
	try{
		const {data}=await client.post<{200:{groups:Group[]}}>({url:'/ns8/directory/groups',body:{password:permissionPassword.value},headers:{'Content-Type':'application/json'}})
		groups.value=data?.groups || [];groupsLoaded.value=true
		if(!groups.value.some(group=>group.id===selectedGroupId.value))selectedGroupId.value=groups.value.find(group=>group.id===props.initialGroupId)?.id || ''
		clearSelection()
	}catch(problem){error.value=errorText(problem);groups.value=[];groupsLoaded.value=false}
	finally{permissionPassword.value='';busy.value=false}
}
async function beginAdd(){
	if(!newMember.value?.id||!selectedGroup.value)return
	pendingChange.value={id:newMember.value.id,name:newMember.value.name || newMember.value.username || '',desired:true,groupId:selectedGroup.value.id,groupName:selectedGroup.value.name}
	await nextTick();confirmInput.value?.focus()
}
async function beginRemove(member:Member){
	if(member.kind!=='direct'||!selectedGroup.value)return
	pendingChange.value={id:member.id,name:member.name || member.username,desired:false,groupId:selectedGroup.value.id,groupName:selectedGroup.value.name}
	await nextTick();confirmInput.value?.focus()
}
function cancelChange(){if(busy.value)return;changePassword.value='';pendingChange.value=null}
function trapConfirmFocus(event:KeyboardEvent){
	const form=event.currentTarget as HTMLFormElement
	const elements=Array.from(form.querySelectorAll<HTMLElement>('input:not(:disabled), button:not(:disabled), a[href]'))
	const first=elements[0], last=elements[elements.length-1]
	if(event.shiftKey&&document.activeElement===first){event.preventDefault();last?.focus()}
	else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first?.focus()}
}
async function confirmChange(){
	if(busy.value||!pendingChange.value||!changePassword.value)return
	const group=groups.value.find(group=>group.id===pendingChange.value?.groupId)
	if(!group)return
	busy.value=true;error.value='';message.value=''
	const change={...pendingChange.value}
	try{
		const {data}=await client.post<{200:ChangeResult}>({url:'/ns8/directory/groups/{group}/membership',path:{group:group.id},
			body:{user_id:change.id,member:change.desired,password:changePassword.value},headers:{'Content-Type':'application/json'}})
		if(!data?.confirmed||data.refresh_pending){
			message.value=t(data?.confirmed?'team.ns8Directory.refreshPending':'team.ns8Directory.unconfirmed')
			groups.value=[];groupsLoaded.value=false
		}else{
			if(data.groups){groups.value=data.groups}
			else if(change.desired){group.members.push({id:change.id,name:change.name,username:newMember.value?.username || '',kind:'direct',disabled:false})}
			else if(data.effective_member){const member=group.members.find(member=>member.id===change.id);if(member)member.kind='inherited'}
			else{group.members=group.members.filter(member=>member.id!==change.id)}
			message.value=t(!change.desired&&data.effective_member?'team.ns8Directory.inheritedRemains':'team.ns8Directory.updated')
			newMember.value=null;userSearch.value=''
			await queryClient.invalidateQueries({queryKey:['teams']})
		}
	}catch(problem){error.value=errorText(problem)}
	finally{changePassword.value='';pendingChange.value=null;busy.value=false}
}
</script>

<style scoped lang="scss">
.directory-groups{margin-block-end:1.5rem}
.permission-check{margin-block:1rem;max-inline-size:32rem}
.directory-confirm-backdrop{position:fixed;inset:0;z-index:2000;background:rgb(0 0 0 / .4);display:flex;align-items:center;justify-content:center;padding:1rem}
.directory-confirm{background:var(--white);color:var(--text);border-radius:.5rem;padding:1.5rem;max-inline-size:38rem;inline-size:100%;box-shadow:0 1rem 3rem rgb(0 0 0 / .2)}
label{display:block;margin-block:.75rem .4rem}
</style>
