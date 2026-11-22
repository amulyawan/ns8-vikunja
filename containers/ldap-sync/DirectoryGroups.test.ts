// SPDX-License-Identifier: AGPL-3.0-or-later
import {mount, flushPromises} from '@vue/test-utils'
import {beforeEach, expect, it, vi} from 'vitest'
import {defineComponent, ref} from 'vue'
import DirectoryGroups from './DirectoryGroups.vue'

const api=vi.hoisted(()=>({get:vi.fn(),post:vi.fn(),invalidate:vi.fn()}))
vi.mock('@/client/generated/client.gen',()=>({client:{get:api.get,post:api.post}}))
vi.mock('@/client/queryClient',()=>({queryClient:{invalidateQueries:api.invalidate}}))
vi.mock('vue-i18n',()=>({useI18n:()=>({t:(key:string,args?:Record<string,string>)=>args?`${key}:${args.name}:${args.group}`:key})}))
vi.mock('@/composables/useUserSearch',()=>({useUserSearch:()=>({users:ref([{id:42,username:'synthetic',name:'Synthetic User'}]),isFetching:ref(false)})}))
vi.mock('@/components/misc/Card.vue',()=>({default:{template:'<section><slot /></section>'}}))
vi.mock('@/components/misc/User.vue',()=>({default:{template:'<span />'}}))
vi.mock('@/components/input/Multiselect.vue',()=>({default:defineComponent({name:'MockMultiselect',props:['modelValue','searchResults','loading'],emits:['update:modelValue','search'],template:'<div />'})}))

const group={id:'group-1',name:'Synthetic Department',description:'',members:[
	{id:3,username:'direct-user',name:'Direct User',kind:'direct',disabled:false},
	{id:4,username:'primary-user',name:'Primary User',kind:'primary',disabled:false},
	{id:5,username:'nested-user',name:'Nested User',kind:'inherited',disabled:false},
]}
beforeEach(()=>{
	api.get.mockReset();api.post.mockReset();api.invalidate.mockReset()
	api.get.mockResolvedValue({data:{enabled:true}})
	api.post.mockImplementation(async(options)=>options.url==='/ns8/directory/groups'
		?{data:{groups:[structuredClone(group)]}}
		:{data:{confirmed:true,changed:true,refresh_pending:false,effective_member:false}})
	api.invalidate.mockResolvedValue(undefined)
})
const Button=defineComponent({props:['disabled'],template:'<button :disabled="disabled"><slot /></button>'})
function setup(){return mount(DirectoryGroups,{global:{components:{XButton:Button}}})}
function button(wrapper:ReturnType<typeof setup>,text:string){return wrapper.findAll('button').find(item=>item.text()===text)!}
async function authorize(wrapper:ReturnType<typeof setup>){
	await flushPromises();await wrapper.find('input[type="password"]').setValue('synthetic-password')
	await button(wrapper,'team.ns8Directory.checkPermissions').trigger('click');await flushPromises()
}

it('requires explicit group selection and clears the permission password',async()=>{
	const wrapper=setup();await authorize(wrapper)
	expect((wrapper.find('input[type="password"]').element as HTMLInputElement).value).toBe('')
	expect((wrapper.find('select').element as HTMLSelectElement).value).toBe('')
	expect(api.post).toHaveBeenCalledWith(expect.objectContaining({url:'/ns8/directory/groups',body:{password:'synthetic-password'}}))
	wrapper.unmount()
})

it('keeps primary and inherited memberships read-only',async()=>{
	const wrapper=setup();await authorize(wrapper);await wrapper.find('select').setValue('group-1')
	const rows=wrapper.findAll('tbody tr')
	expect(rows[0].find('button').attributes('disabled')).toBeUndefined()
	expect(rows[1].find('button').attributes('disabled')).toBeDefined()
	expect(rows[2].find('button').attributes('disabled')).toBeDefined()
	expect(api.post).toHaveBeenCalledTimes(1);wrapper.unmount()
})

it('adds only after the user confirms the exact group and enters their password',async()=>{
	const wrapper=setup();await authorize(wrapper);await wrapper.find('select').setValue('group-1')
	wrapper.findComponent({name:'MockMultiselect'}).vm.$emit('update:modelValue',{id:42,username:'synthetic',name:'Synthetic User'})
	await flushPromises();await button(wrapper,'team.edit.addUser').trigger('click');await flushPromises()
	expect(api.post).toHaveBeenCalledTimes(1)
	expect(wrapper.find('[role="dialog"]').text()).toContain('Synthetic User:Synthetic Department')
	await wrapper.find('[role="dialog"] input').setValue('confirmation-password')
	await button(wrapper,'team.ns8Directory.confirm').trigger('click');await flushPromises()
	expect(api.post).toHaveBeenLastCalledWith(expect.objectContaining({url:'/ns8/directory/groups/{group}/membership',path:{group:'group-1'},body:{user_id:42,member:true,password:'confirmation-password'}}))
	expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
	expect((wrapper.find('input[type="password"]').element as HTMLInputElement).value).toBe('')
	expect(api.post.mock.calls.every(([options])=>options.url.startsWith('/ns8/directory/'))).toBe(true)
	wrapper.unmount()
})

it('cancels removal without changing the directory',async()=>{
	const wrapper=setup();await authorize(wrapper);await wrapper.find('select').setValue('group-1')
	await wrapper.find('tbody tr button').trigger('click');await flushPromises()
	expect(wrapper.find('[role="dialog"]').text()).toContain('Direct User:Synthetic Department')
	await button(wrapper,'misc.cancel').trigger('click');await flushPromises()
	expect(api.post).toHaveBeenCalledTimes(1);expect(wrapper.find('[role="dialog"]').exists()).toBe(false);wrapper.unmount()
})
